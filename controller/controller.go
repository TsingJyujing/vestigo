package controller

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/coder/hnsw"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
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
	for modeName, _ := range embeddingModels {
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
func handleSQLError(echoCtx echo.Context, err error) error {
	if err.Error() == RowNotFoundMessage {
		return echoCtx.JSON(http.StatusNotFound, map[string]string{"status": "not found"})
	} else {
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
	}
}

func handleGenericError(echoCtx echo.Context, err error, status int) error {
	logger.WithError(err).WithField("status", status).Error("Error handling request")
	return echoCtx.JSON(status, map[string]string{"status": err.Error()})
}

func handleInternalError(echoCtx echo.Context, err error) error {
	return handleGenericError(echoCtx, err, http.StatusInternalServerError)
}

type NewDocumentParams struct {
	ID          string                 `json:"id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description"`
	Data        map[string]interface{} `json:"data"`
	Texts       []string               `json:"texts"`
}

func (c *Controller) NewDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()

	param := NewDocumentParams{}
	if err := echoCtx.Bind(&param); err != nil {
		return handleGenericError(echoCtx, err, http.StatusBadRequest)
	}

	// Check if overwrite parameter is set
	overwrite := echoCtx.QueryParam("overwrite")
	shouldOverwrite := overwrite == "true" || overwrite == "1"

	// AI summarization enabled?
	aiSummarize := echoCtx.QueryParam("ai_sum")
	if (aiSummarize == "true" || aiSummarize == "1") && len(param.Texts) > 0 && c.summarizeModel != nil {
		summarizedText, err := c.summarizeModel.Summarize(ctx, param.Texts)
		if err != nil {
			logger.WithError(err).Error("failed to summarize document texts")
		} else {
			param.Texts = append(param.Texts, summarizedText)
			logger.WithField("texts", summarizedText).Info("summarized document texts") // TODO remove debug log after finished
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
					if err := deleteDocumentInternal(ctx, queries, param.ID); err != nil {
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
	return echoCtx.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

type DocumentWithChunks struct {
	dao.Document
	Texts []dao.TextChunk `json:"texts,omitempty"`
}

func (c *Controller) GetDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	doc, err := c.queries.GetDocument(ctx, docId)
	if err != nil {
		return handleSQLError(echoCtx, err)
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
		response := DocumentWithChunks{
			Document: doc,
			Texts:    texts,
		}
		return echoCtx.JSON(http.StatusOK, response)
	}
	// Return document without chunks
	return echoCtx.JSON(http.StatusOK, doc)
}

// deleteDocumentInternal deletes a document and all related text chunks / embeddings
// This is an internal helper function used by both DeleteDocument and NewDocument (with overwrite)
func deleteDocumentInternal(ctx context.Context, queries *dao.Queries, docId string) error {
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
	return nil
}

// DeleteDocument deletes a document and all related text chunks / embeddings
func (c *Controller) DeleteDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	_, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (any, error) {
			queries := dao.New(tx)
			if err := deleteDocumentInternal(ctx, queries, docId); err != nil {
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

func (c *Controller) NewTextChunk(echoCtx echo.Context) error {
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
	textChunk, err := utils.WithTx(
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
	return echoCtx.JSON(http.StatusCreated, textChunk)
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
	logger.Infof("New segment content: %s", segContent) // TODO remove debug log after finished
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

func (c *Controller) GetTextChunk(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	textId := echoCtx.Param("text_id")
	textChunk, err := c.queries.GetTextChunk(ctx, textId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, textChunk)
}

func (c *Controller) DeleteTextChunk(echoCtx echo.Context) error {
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
	Rank        float64 `json:"rank" jsonschema:"the rank score of the search result"`
}

func (c *Controller) SimpleSearch(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	query := echoCtx.QueryParam("q")
	if query == "" {
		return handleGenericError(echoCtx, echo.NewHTTPError(http.StatusBadRequest, "query parameter 'q' is required"), http.StatusBadRequest)
	}
	nDoc, err := strconv.Atoi(echoCtx.QueryParam("n"))
	if err != nil || nDoc <= 0 {
		nDoc = 10
	}

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
		return handleInternalError(echoCtx, err)
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
		if err := rows.Scan(&item.TextChunkID, &item.Content, &item.DocumentID, &item.Title, &item.Description, &item.Rank); err != nil {
			return handleInternalError(echoCtx, err)
		}
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return handleInternalError(echoCtx, err)
	}

	return echoCtx.JSON(http.StatusOK, results)
}

type ANNSearchTextChunkResult struct {
	ID         string
	DocumentID string
	Content    string
	CreatedAt  int64
}

func (c *Controller) ANNSearch(echoCtx echo.Context) error {
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
	ctx := echoCtx.Request().Context()
	idx, ok := c.embeddingIndexes[modelId]
	model, okModel := c.embeddingModels[modelId]
	if !ok {
		return handleGenericError(echoCtx, fmt.Errorf("embedding index '%s' not found", modelId), http.StatusNotFound)
	}
	if !okModel {
		return handleGenericError(echoCtx, fmt.Errorf("embedding model '%s' not found", modelId), http.StatusNotFound)
	}
	queryEmbedding, err := model.Embed(ctx, []string{query})
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	if len(queryEmbedding) != 1 {
		return handleGenericError(echoCtx, fmt.Errorf("embedding model returned unexpected number of embeddings: %d", len(queryEmbedding)), http.StatusInternalServerError)
	}
	var ids = lo.Map(idx.Search(queryEmbedding[0], nDoc), func(item hnsw.Node[string], index int) any {
		return item.Key
	})
	sqlStat := fmt.Sprintf(
		"SELECT * FROM text_chunk WHERE id IN (%s)", strings.Join(
			lo.Map(ids, func(item any, index int) string {
				return "?"
			}),
			",",
		),
	)
	rows, err := c.db.QueryContext(ctx, sqlStat, ids...)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {
			logger.WithError(err).Error("Failed to close rows")
		}
	}(rows)

	results := make([]dao.TextChunk, 0) // TODO add score to result, remove unnecessary fields
	for rows.Next() {
		var item dao.TextChunk
		if err := rows.Scan(&item.ID, &item.DocumentID, &item.Content, &item.SegContent, &item.CreatedAt); err != nil {
			return handleInternalError(echoCtx, err)
		}
		results = append(results, item)
	}
	if err := rows.Err(); err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, results)
}
