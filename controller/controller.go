package controller

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/coder/hnsw"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/tsingjyujing/vestigo/controller/dao"
	"github.com/tsingjyujing/vestigo/models"
	"github.com/tsingjyujing/vestigo/text"
	"github.com/tsingjyujing/vestigo/utils"
)

//go:embed sqlc/schema.sql
var ddl string

func GetDDL() string {
	return ddl
}

const (
	RowNotFoundMessage = "sql: no rows in result set"
)

var logger = logrus.New()

type Controller struct {
	db               *sql.DB
	queries          dao.Queries
	tokenizer        text.Tokenizer
	normalizer       text.Normalizer
	embeddingModels  map[string]models.BaseEmbeddingModel
	embeddingIndexes map[string]*hnsw.SavedGraph[string]
	summarizeModel   models.SummarizationModel
}

func NewController(db *sql.DB, embeddingModels map[string]models.BaseEmbeddingModel, embeddingSavePath string, summarizeModel models.SummarizationModel) (*Controller, error) {
	tokenizer, err := text.NewGSETokenizer(true)
	if err != nil {
		return nil, err
	}
	normalizer, err := text.NewCJKNormalizer(true, true)
	if err != nil {
		return nil, err
	}
	embeddingIndexes := make(map[string]*hnsw.SavedGraph[string])
	for modeName := range embeddingModels {
		// TODO check embedding/...
		graph, err := hnsw.LoadSavedGraph[string](filepath.Join(embeddingSavePath, fmt.Sprintf("%s.embed", modeName)))
		// TODO Set parameters, cosine/M/...
		if err != nil {
			return nil, err
		}
		embeddingIndexes[modeName] = graph
	}
	return &Controller{
		queries:          *dao.New(db),
		db:               db,
		tokenizer:        tokenizer,
		normalizer:       normalizer,
		embeddingModels:  embeddingModels,
		embeddingIndexes: embeddingIndexes,
		summarizeModel:   summarizeModel,
	}, nil
}

// Close closes all resources held by the controller
func (c *Controller) Close() error {
	for modelId, graph := range c.embeddingIndexes {
		err := graph.Save()
		if err != nil {
			logger.WithError(err).Errorf("Failed to save embedding index for model %s", modelId)
		}
	}
	if err := c.db.Close(); err != nil {
		logger.WithError(err).Error("Failed to close database")
	}
	logger.Info("Controller resources closed successfully")
	return nil
}

// handleSQLError return http response by error return from sql
func handleSQLError(echoCtx *echo.Context, err error) error {
	if err.Error() == RowNotFoundMessage {
		return (*echoCtx).JSON(http.StatusNotFound, map[string]string{"status": "not found"})
	} else {
		return (*echoCtx).JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
	}
}

func handleGenericError(echoCtx *echo.Context, err error, status int) error {
	logger.WithError(err).WithField("status", status).Error("Error handling request")
	return (*echoCtx).JSON(status, map[string]string{"status": err.Error()})
}

func handleInternalError(echoCtx *echo.Context, err error) error {
	return handleGenericError(echoCtx, err, http.StatusInternalServerError)
}

func returnJsonResponse(echoCtx *echo.Context, data any, status int) error {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return (*echoCtx).JSONBlob(status, jsonString)
}

type NewDocumentParams struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	Texts       []string               `json:"texts"`
}

func (c *Controller) NewDocument(echoCtx *echo.Context) error {
	ctx := (*echoCtx).Request().Context()

	param := NewDocumentParams{}
	if err := (*echoCtx).Bind(&param); err != nil {
		return handleGenericError(echoCtx, err, http.StatusBadRequest)
	}

	// Check if overwrite parameter is set
	overwrite := (*echoCtx).QueryParam("overwrite")
	shouldOverwrite := overwrite == "true" || overwrite == "1"

	// AI summarization enabled?
	aiSummarize := (*echoCtx).QueryParam("ai_sum")
	if (aiSummarize == "true" || aiSummarize == "1") && len(param.Texts) > 0 && c.summarizeModel != nil {
		summarizedText, err := c.summarizeModel.Summarize(ctx, param.Texts)
		if err != nil {
			logger.WithError(err).Error("failed to summarize document texts")
		} else {
			param.Texts = append(param.Texts, summarizedText)
			logger.WithField("texts", summarizedText).Debugf("summarized document texts")
		}
	}

	insertCount, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (int, error) {
			queries := dao.New(tx)

			// If overwrite is enabled, delete existing document first
			if shouldOverwrite {
				// Check if document exists
				_, err := queries.GetDocument(ctx, param.ID)
				if err == nil {
					// Document exists, delete it using the shared internal function
					if err := c.deleteDocumentInternal(ctx, queries, param.ID); err != nil {
						return 0, err
					}
					logger.WithField("document_id", param.ID).Info("Deleted existing document for overwrite")
				}
				// If document doesn't exist, ignore the error and continue to create
			}

			// Convert data map to JSON string, default to empty object
			dataJSON := "{}"
			if len(param.Data) > 0 {
				jsonBytes, err := json.Marshal(param.Data)
				if err != nil {
					return 0, err
				}
				dataJSON = string(jsonBytes)
			}
			err := queries.NewDocument(ctx, dao.NewDocumentParams{
				ID:          param.ID,
				Title:       param.Title,
				Description: param.Description,
				Data:        dataJSON,
			})
			if err != nil {
				return 0, err
			}
			textChunks := make([]*dao.TextChunk, 0, len(param.Texts))
			for _, t := range param.Texts {
				tc, err := c.createTextChunks(ctx, param.ID, tx, t)
				if err != nil {
					return 0, err
				}
				textChunks = append(textChunks, tc)
			}
			return len(textChunks), nil
		},
	)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	logger.WithField("inserted_text_chunks", insertCount).Debug("Inserted text chunks for new document")
	return (*echoCtx).JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

type Document struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Data        map[string]any `json:"data"`
	CreatedAt   int64          `json:"created_at"`
}
type DocumentWithChunks struct {
	Document
	Texts []dao.TextChunk `json:"texts,omitempty"`
}

func (c *Controller) GetDocument(echoCtx *echo.Context) error {
	ctx := (*echoCtx).Request().Context()
	docId := (*echoCtx).Param("doc_id")
	// unquote docId
	docId, err := url.QueryUnescape(docId)
	if err != nil {
		return handleGenericError(echoCtx, err, http.StatusBadRequest)
	}
	row, err := c.queries.GetDocument(ctx, docId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	// Convert data JSON string to map
	dataMap := make(map[string]any)
	if err := json.Unmarshal([]byte(row.Data), &dataMap); err != nil {
		return handleInternalError(echoCtx, err)
	}
	// Prepare document response
	document := Document{
		ID:          row.ID,
		Title:       row.Title,
		Description: row.Description,
		Data:        dataMap,
		CreatedAt:   row.CreatedAt,
	}

	// Check if with_chunks parameter is set
	withChunks := echoCtx.QueryParam("with_texts")
	if withChunks == "true" || withChunks == "1" {
		// Fetch all text texts for this document
		texts, err := c.queries.ListTextChunksByDocumentID(ctx, docId)
		if err != nil {
			return handleInternalError(echoCtx, err)
		}

		// Return document with chunks
		documentWithChunks := DocumentWithChunks{
			Document: document,
			Texts:    texts,
		}
		return returnJsonResponse(echoCtx, documentWithChunks, http.StatusOK)
	}
	// Return document without chunks
	return returnJsonResponse(echoCtx, document, http.StatusOK)
}

// deleteDocumentInternal deletes a document and all related text chunks / embeddings
// This is an internal helper function used by both DeleteDocument and NewDocument (with overwrite)
func (c *Controller) deleteDocumentInternal(ctx context.Context, queries *dao.Queries, docId string) error {
	textChunkIds, err := queries.ListTextChunkIdByDocumentID(ctx, docId)
	if err != nil {
		return err
	}
	// Delete text embeddings associated with this document's chunks
	if err := queries.DeleteTextEmbeddingsByDocumentID(ctx, docId); err != nil {
		return err
	}
	// Delete FTS entries
	if err := queries.DeleteTextChunkFTSByDocumentID(ctx, docId); err != nil {
		return err
	}
	// Delete text chunks
	if err := queries.DeleteTextChunksByDocumentID(ctx, docId); err != nil {
		return err
	}
	// Delete document
	if err := queries.DeleteDocument(ctx, docId); err != nil {
		return err
	}
	for _, textChunkId := range textChunkIds {
		c.deleteTextChunkFromIndex(textChunkId)
	}
	return nil
}

// DeleteDocument deletes a document and all related text chunks / embeddings
func (c *Controller) DeleteDocument(echoCtx *echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	_, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (any, error) {
			if err := c.deleteDocumentInternal(ctx, dao.New(tx), docId); err != nil {
				return nil, err
			}
			return nil, nil
		},
	)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type TextChunk struct {
	ID         string `json:"id"`
	DocumentID string `json:"document_id"`
	Content    string `json:"content"`
	SegContent string `json:"seg_content"`
	CreatedAt  int64  `json:"created_at"`
}

func (c *Controller) NewTextChunk(echoCtx *echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	//  validate document ID before creating text chunk
	if _, err := c.queries.GetDocument(ctx, docId); err != nil {
		return handleSQLError(echoCtx, err)
	}
	param := &struct {
		Content string `json:"content"`
	}{}
	if err := echoCtx.Bind(&param); err != nil {
		return handleGenericError(echoCtx, err, http.StatusBadRequest)
	}
	row, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (*dao.TextChunk, error) {
			return c.createTextChunks(ctx, docId, tx, param.Content)
		},
	)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusCreated, TextChunk{
		ID:         row.ID,
		DocumentID: row.DocumentID,
		Content:    row.Content,
		SegContent: row.SegContent,
		CreatedAt:  row.CreatedAt,
	})
}

func (c *Controller) createTextChunks(ctx context.Context, docId string, tx *sql.Tx, text string) (*dao.TextChunk, error) {
	newUUID, uuidErr := uuid.NewRandom()
	if uuidErr != nil {
		return nil, uuidErr
	}
	tokenizedText := c.tokenizer.Tokenize(text)
	tokenizedNormalizedText := lo.Map(tokenizedText, func(item string, index int) string {
		normText, err := c.normalizer.Normalize(item)
		if err != nil {
			logger.WithError(err).Error("Failed to normalize text")
			return item
		}
		return normText
	})
	segContent := strings.Join(append(tokenizedText, tokenizedNormalizedText...), " ")
	logger.Debugf("New segment content: %s", segContent)
	requestParam := dao.NewTextChunkParams{
		DocumentID: docId,
		Content:    text,
		ID:         newUUID.String(),
		SegContent: segContent,
	}
	queries := dao.New(tx)
	newText, err := queries.NewTextChunk(ctx, requestParam)
	if err != nil {
		return nil, err
	}
	// Add to FTS5 table using generated method
	if err := queries.InsertTextChunkFTS(ctx, dao.InsertTextChunkFTSParams{
		ID:         newText.ID,
		SegContent: newText.SegContent,
	}); err != nil {
		return nil, err
	}
	for modelId, graph := range c.embeddingIndexes {
		model := c.embeddingModels[modelId]
		embeddings, err := model.Embed(ctx, []string{text})
		if err != nil {
			return nil, err
		}
		if len(embeddings) != 1 {
			return nil, fmt.Errorf("embedding model returned unexpected number of embeddings: %d", len(embeddings))
		}
		graph.Add(hnsw.Node[string]{
			Key:   newText.ID,
			Value: embeddings[0],
		})
		err = queries.NewTextEmbedding(ctx, dao.NewTextEmbeddingParams{
			TextChunkID: newText.ID,
			ModelID:     modelId,
			Vector:      utils.ConvertFloat32ArrayToBytes(embeddings[0]),
		})
		if err != nil {
			return nil, err
		}
	}
	return &newText, nil
}

func (c *Controller) GetTextChunk(echoCtx *echo.Context) error {
	ctx := echoCtx.Request().Context()
	textId := echoCtx.Param("text_id")
	row, err := c.queries.GetTextChunk(ctx, textId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, TextChunk{
		ID:         row.ID,
		DocumentID: row.DocumentID,
		Content:    row.Content,
		SegContent: row.SegContent,
		CreatedAt:  row.CreatedAt,
	})
}

func (c *Controller) deleteTextChunkFromIndex(id string) {
	for modelId, graph := range c.embeddingIndexes {
		if !graph.Delete(id) {
			logger.Errorf("Failed to delete text chunk %s from embedding index %s", id, modelId)
		}
	}
}

func (c *Controller) DeleteTextChunk(echoCtx *echo.Context) error {
	ctx := echoCtx.Request().Context()
	textId := echoCtx.Param("text_id")
	_, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (any, error) {
			queries := dao.New(tx)
			// Delete text embeddings
			if err := queries.DeleteTextEmbeddingsByTextChunkID(ctx, textId); err != nil {
				return nil, err
			}
			// Delete FTS entry
			if err := queries.DeleteTextChunkFTSByID(ctx, textId); err != nil {
				return nil, err
			}
			// Delete text chunk
			if err := queries.DeleteTextChunk(ctx, textId); err != nil {
				return nil, err
			}
			c.deleteTextChunkFromIndex(textId)
			return nil, nil
		},
	)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type SearchResultItem struct {
	TextChunkID string  `json:"text_chunk_id" jsonschema:"the ID of the text chunk"`
	Content     string  `json:"content" jsonschema:"the content of the text chunk"`
	DocumentID  string  `json:"document_id" jsonschema:"the ID of the document"`
	Title       string  `json:"title" jsonschema:"the title of the document"`
	Description string  `json:"description" jsonschema:"the description of the document"`
	Score       float64 `json:"score" jsonschema:"the score score of the search result"`
}

func (c *Controller) searchWithBM25(ctx context.Context, query string, nDoc int) ([]SearchResultItem, error) {
	rows, err := c.db.QueryContext(ctx, `
		SELECT 
			tc.id,
			tc.content,
			tc.document_id,
			d.title,
			d.description,
			fts.rank
		FROM text_chunk_fts fts
		JOIN text_chunk tc ON tc.id = fts.id
		JOIN document d ON d.id = tc.document_id
		WHERE fts.seg_content MATCH ?
		ORDER BY fts.rank
		LIMIT ?
	`, query, nDoc)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.WithError(err).Error("Failed to close rows")
		}
	}(rows)

	results := make([]SearchResultItem, 0)
	for rows.Next() {
		var item SearchResultItem
		if err := rows.Scan(&item.TextChunkID, &item.Content, &item.DocumentID, &item.Title, &item.Description, &item.Score); err != nil {
			return nil, err
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

func (c *Controller) searchWithEmbeddingModel(ctx context.Context, modelId, query string, nDoc int) ([]SearchResultItem, error) {
	index := c.embeddingIndexes[modelId]
	model := c.embeddingModels[modelId]
	queryEmbedding, err := model.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(queryEmbedding) != 1 {
		return nil, fmt.Errorf("embedding model returned unexpected number of embeddings: %d", len(queryEmbedding))
	}
	searchResult := index.SearchWithDistance(queryEmbedding[0], nDoc)
	ids := lo.Map(searchResult, func(item hnsw.SearchResult[string], index int) any {
		return item.Key
	})
	distanceMap := make(map[string]float32)
	for _, item := range searchResult {
		distanceMap[item.Key] = item.Distance
	}
	sqlStat := fmt.Sprintf(
		`SELECT 
				tc.id,
				tc.content,
				tc.document_id,
				d.title,
				d.description
			FROM text_chunk tc
			JOIN document d ON d.id = tc.document_id
			WHERE tc.id IN (%s)
			`, strings.Join(
			lo.Map(ids, func(item any, index int) string {
				return "?"
			}),
			",",
		),
	)
	rows, err := c.db.QueryContext(ctx, sqlStat, ids...)
	if err != nil {
		return nil, err
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.WithError(err).Error("Failed to close rows")
		}
	}(rows)
	results := make([]SearchResultItem, 0)
	for rows.Next() {
		var item SearchResultItem
		if err := rows.Scan(&item.TextChunkID, &item.Content, &item.DocumentID, &item.Title, &item.Description); err != nil {
			return nil, err
		}
		item.Score = -float64(distanceMap[item.TextChunkID])
		results = append(results, item)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return results, nil
}

type SearchResponse struct {
	Results []SearchResultItem `json:"results"`
}

func (c *Controller) Search(echoCtx *echo.Context) error {
	ctx := echoCtx.Request().Context()
	modelId := echoCtx.Param("model_id")
	query := echoCtx.QueryParam("q")
	nDocParam := echoCtx.QueryParam("n")
	if query == "" {
		return handleGenericError(echoCtx, echo.NewHTTPError(http.StatusBadRequest, "query parameter 'q' is required"), http.StatusBadRequest)
	}
	nDoc, err := strconv.Atoi(nDocParam)
	if err != nil || nDoc <= 0 {
		nDoc = 10
	}
	var results []SearchResultItem
	if strings.ToLower(modelId) == "bm25" {
		results, err = c.searchWithBM25(ctx, query, nDoc)
		if err != nil {
			return handleInternalError(echoCtx, err)
		}
	} else {
		// Check if model exists
		_, okModel := c.embeddingModels[modelId]
		_, okAnnIndex := c.embeddingIndexes[modelId]
		if !okModel || !okAnnIndex {
			return handleGenericError(echoCtx, echo.NewHTTPError(http.StatusBadRequest, fmt.Sprintf("model '%s' not found", modelId)), http.StatusBadRequest)
		}
		results, err = c.searchWithEmbeddingModel(ctx, modelId, query, nDoc)
		if err != nil {
			return handleInternalError(echoCtx, err)
		}
	}
	return returnJsonResponse(echoCtx, SearchResponse{Results: results}, http.StatusOK)
}

func (c *Controller) ListEmbeddingModels(echoCtx *echo.Context) error {
	modelIds := make([]string, 0, len(c.embeddingModels))
	for modelId := range c.embeddingModels {
		modelIds = append(modelIds, modelId)
	}
	return echoCtx.JSON(http.StatusOK, modelIds)
}
