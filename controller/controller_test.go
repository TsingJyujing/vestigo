package controller

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

// setupTestController creates a controller with an in-memory database
func setupTestController(t *testing.T) (*Controller, *sql.DB) {
	// Create in-memory database
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err, "Failed to open in-memory database")

	// Create tables using DDL
	_, err = db.Exec(GetDDL())
	require.NoError(t, err, "Failed to create tables")

	// Create controller
	controller, err := NewController(db)
	require.NoError(t, err, "Failed to create controller")

	return controller, db
}

// TestDocumentCRUDAndSearch tests creating, searching, and deleting documents
func TestDocumentCRUDAndSearch(t *testing.T) {
	controller, db := setupTestController(t)
	defer db.Close()

	e := echo.New()

	// Test data - based on debug.http
	documents := []struct {
		ID          string
		Title       string
		Description string
		Data        map[string]interface{}
		Texts       []string
	}{
		{
			ID:          "doc-test-001",
			Title:       "山达尔星联邦介绍",
			Description: "关于山达尔星联邦共和国的基本信息",
			Data: map[string]interface{}{
				"author":   "测试作者",
				"category": "政治",
				"version":  1,
			},
			Texts: []string{
				"山达尔星联邦共和国联邦政府是一个强大的政治实体",
				"它由多个星球组成，共同致力于维护和平与繁荣",
			},
		},
		{
			ID:          "doc-test-002",
			Title:       "星际贸易协定",
			Description: "山达尔星与邻近星系的贸易关系",
			Data:        map[string]interface{}{"author": "贸易部", "category": "经济"},
			Texts: []string{
				"星际贸易协定促进了各星球之间的经济交流",
				"山达尔星作为贸易中心，吸引了大量商业活动",
			},
		},
		{
			ID:          "doc-test-003",
			Title:       "联邦科技发展",
			Description: "山达尔星联邦的科技创新成就",
			Data: map[string]interface{}{
				"author":   "科技部",
				"category": "科技",
				"priority": "high",
			},
			Texts: []string{
				"联邦政府大力投资科技研发项目",
				"先进的空间跳跃技术使得星际旅行更加便捷",
				"能源革命为各星球提供了清洁可持续的动力",
			},
		},
		{
			ID:          "doc-test-004",
			Title:       "和平维护机制",
			Description: "联邦如何维持星际和平",
			Data:        nil, // Test with nil data, should default to {}
			Texts: []string{
				"联邦建立了完善的和平维护机制",
				"各成员星球通过民主协商解决争端",
				"军事力量仅用于防御外部威胁",
			},
		},
	}

	// Step 1: Create documents
	t.Run("CreateDocuments", func(t *testing.T) {
		for _, doc := range documents {
			reqBody, err := json.Marshal(NewDocumentParams{
				ID:          doc.ID,
				Title:       doc.Title,
				Description: doc.Description,
				Data:        doc.Data,
				Texts:       doc.Texts,
			})
			require.NoError(t, err)

			req := httptest.NewRequest(http.MethodPost, "/api/v1/doc/", bytes.NewReader(reqBody))
			req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err = controller.NewDocument(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusCreated, rec.Code, "Failed to create document: %s", doc.ID)
		}
	})

	// Step 2: Get documents to verify creation
	t.Run("GetDocuments", func(t *testing.T) {
		for _, doc := range documents {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/doc/"+doc.ID, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("doc_id")
			c.SetParamValues(doc.ID)

			err := controller.GetDocument(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			// Verify the response contains expected data
			var response map[string]interface{}
			err = json.Unmarshal(rec.Body.Bytes(), &response)
			require.NoError(t, err)
			assert.Equal(t, doc.ID, response["ID"])
			assert.Equal(t, doc.Title, response["Title"])
		}
	})

	// Step 3: Search documents
	t.Run("SearchDocuments", func(t *testing.T) {
		searchTests := []struct {
			query          string
			minExpected    int
			expectedInText string
		}{
			{"山", 2, "山"},
			{"联邦", 2, "联邦"},
			{"和平", 1, "和平"},
			{"科技", 1, "科技"},
			{"星球", 2, "星球"},
		}

		for _, st := range searchTests {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q="+st.query, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)

			err := controller.SimpleSearch(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code)

			var results []SearchResultItem
			err = json.Unmarshal(rec.Body.Bytes(), &results)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(results), st.minExpected, "Search for '%s' should return at least %d results", st.query, st.minExpected)

			// Verify at least one result contains the expected text
			if len(results) > 0 {
				found := false
				for _, result := range results {
					if containsText(result.Content, st.expectedInText) ||
						containsText(result.Title, st.expectedInText) {
						found = true
						break
					}
				}
				assert.True(t, found, "Search results should contain text: %s", st.expectedInText)
			}
		}
	})

	// Step 4: Delete documents
	t.Run("DeleteDocuments", func(t *testing.T) {
		for _, doc := range documents {
			req := httptest.NewRequest(http.MethodDelete, "/api/v1/doc/"+doc.ID, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("doc_id")
			c.SetParamValues(doc.ID)

			err := controller.DeleteDocument(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, rec.Code, "Failed to delete document: %s", doc.ID)
		}
	})

	// Step 5: Verify documents are deleted
	t.Run("VerifyDocumentsDeleted", func(t *testing.T) {
		for _, doc := range documents {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/doc/"+doc.ID, nil)
			rec := httptest.NewRecorder()
			c := e.NewContext(req, rec)
			c.SetParamNames("doc_id")
			c.SetParamValues(doc.ID)

			err := controller.GetDocument(c)
			require.NoError(t, err)
			assert.Equal(t, http.StatusNotFound, rec.Code, "Document should not exist after deletion: %s", doc.ID)
		}
	})

	// Step 6: Verify search returns no results after deletion
	t.Run("SearchAfterDeletion", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=山达尔星", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 0, len(results), "Search should return no results after all documents are deleted")
	})
}

// Helper function to check if text contains a substring
func containsText(text, substr string) bool {
	return len(text) > 0 && len(substr) > 0 && (text == substr || bytes.Contains([]byte(text), []byte(substr)))
}

// TestSearchWithLimitParameter tests the n parameter for limiting search results
func TestSearchWithLimitParameter(t *testing.T) {
	controller, db := setupTestController(t)
	defer db.Close()

	e := echo.New()

	// Create multiple documents to test limit functionality
	for i := 1; i <= 10; i++ {
		reqBody, err := json.Marshal(NewDocumentParams{
			ID:          "doc-limit-test-" + string(rune('0'+i)),
			Title:       "测试文档",
			Description: "测试限制参数",
			Data:        map[string]interface{}{"index": i},
			Texts:       []string{"这是一个包含测试关键词的文档内容"},
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/doc/", bytes.NewReader(reqBody))
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err = controller.NewDocument(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusCreated, rec.Code)
	}

	// Test with default limit (should return all 10 results since default is 100)
	t.Run("SearchWithDefaultLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results), "Should return all 10 results with default limit")
	})

	// Test with n=5 (should return exactly 5 results)
	t.Run("SearchWithLimit5", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试&n=5", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 5, len(results), "Should return exactly 5 results when n=5")
	})

	// Test with n=1 (should return exactly 1 result)
	t.Run("SearchWithLimit1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试&n=1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 1, len(results), "Should return exactly 1 result when n=1")
	})

	// Test with n=0 or invalid (should use default limit of 100)
	t.Run("SearchWithInvalidLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试&n=0", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results), "Should return all 10 results when n=0 (falls back to default)")
	})

	// Test with n=-1 (should use default limit of 100)
	t.Run("SearchWithNegativeLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试&n=-1", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results), "Should return all 10 results when n=-1 (falls back to default)")
	})

	// Test with n=abc (invalid string, should use default limit)
	t.Run("SearchWithNonNumericLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试&n=abc", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results), "Should return all 10 results when n=abc (falls back to default)")
	})

	// Test with very large n (should return all available results)
	t.Run("SearchWithLargeLimit", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/search/simple?q=测试&n=1000", nil)
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)

		err := controller.SimpleSearch(c)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, rec.Code)

		var results []SearchResultItem
		err = json.Unmarshal(rec.Body.Bytes(), &results)
		require.NoError(t, err)
		assert.Equal(t, 10, len(results), "Should return all 10 available results when n=1000")
	})
}

// TestDocumentWithEmptyData tests creating a document with nil/empty data
func TestDocumentWithEmptyData(t *testing.T) {
	controller, db := setupTestController(t)
	defer db.Close()

	e := echo.New()

	// Create document with nil data
	reqBody, err := json.Marshal(NewDocumentParams{
		ID:          "doc-empty-data",
		Title:       "测试空数据",
		Description: "测试data字段为空时的默认值",
		Data:        nil,
		Texts:       []string{"这是一个测试文本"},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/doc/", bytes.NewReader(reqBody))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	err = controller.NewDocument(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Get the document and verify data field is "{}"
	req = httptest.NewRequest(http.MethodGet, "/api/v1/doc/doc-empty-data", nil)
	rec = httptest.NewRecorder()
	c = e.NewContext(req, rec)
	c.SetParamNames("doc_id")
	c.SetParamValues("doc-empty-data")

	err = controller.GetDocument(c)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	var response map[string]interface{}
	err = json.Unmarshal(rec.Body.Bytes(), &response)
	require.NoError(t, err)
	assert.Equal(t, "{}", response["Data"], "Data field should default to empty JSON object")
}
