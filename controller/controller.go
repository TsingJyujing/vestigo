package controller

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/samber/lo"
	"github.com/sirupsen/logrus"
	"github.com/tsingjyujing/vestigo/controller/dao"
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
	db         *sql.DB
	queries    dao.Queries
	tokenizer  text.Tokenizer
	normalizer text.Normalizer
}

func NewController(db *sql.DB) (*Controller, error) {
	tokenizer, err := text.NewGSETokenizer(true)
	if err != nil {
		return nil, err
	}
	normalizer, err := text.NewCJKNormalizer(true, true)
	if err != nil {
		return nil, err
	}
	return &Controller{
		queries:    *dao.New(db),
		db:         db,
		tokenizer:  tokenizer,
		normalizer: normalizer,
	}, nil
}

// Close closes all resources held by the controller
func (c *Controller) Close() error {
	// TODO close search indexes
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
	insertCount, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (int, error) {
			// Convert data map to JSON string, default to empty object
			dataJSON := "{}"
			if len(param.Data) > 0 {
				jsonBytes, err := json.Marshal(param.Data)
				if err != nil {
					return 0, err
				}
				dataJSON = string(jsonBytes)
			}

			err := dao.New(tx).NewDocument(ctx, dao.NewDocumentParams{
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
	logger.WithField("inserted_text_chunks", insertCount).Info("Inserted text chunks for new document")
	return echoCtx.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

func (c *Controller) GetDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	doc, err := c.queries.GetDocument(ctx, docId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, doc) // TODO add all text chunks by config
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
			_, err := tx.ExecContext(ctx, `
				DELETE FROM text_embedding 
				WHERE text_chunk_id IN (
					SELECT id FROM text_chunk tc
					WHERE tc.document_id = ?
				)
			`, docId)
			if err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `
				DELETE FROM text_chunk_fts 
				WHERE id IN (SELECT id FROM text_chunk tc WHERE tc.document_id = ?)`, docId)
			if err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `DELETE FROM text_chunk WHERE document_id = ?`, docId)
			if err != nil {
				return nil, err
			}
			err = dao.New(tx).DeleteDocument(ctx, docId)
			return nil, err
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
	logger.Infof("New segment content: %s", segContent)
	requestParam := dao.NewTextChunkParams{
		DocumentID: docId,
		Content:    text,
		ID:         newUUID.String(),
		SegContent: segContent,
	}
	newText, err := dao.New(tx).NewTextChunk(ctx, requestParam)
	if err != nil {
		return nil, err
	}
	// Add to FTS5 table
	_, err = tx.ExecContext(ctx, "INSERT INTO text_chunk_fts (id, seg_content) VALUES (?, ?)", newText.ID, newText.SegContent)
	if err != nil {
		return nil, err
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
			_, err := tx.ExecContext(ctx, `DELETE FROM text_embedding WHERE text_chunk_id = ?`, textId)
			if err != nil {
				return nil, err
			}
			_, err = tx.ExecContext(ctx, `DELETE FROM text_chunk_fts WHERE id = ?`, textId)
			if err != nil {
				return nil, err
			}
			err = dao.New(tx).DeleteTextChunk(ctx, textId)
			return nil, err
		},
	)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type SearchResultItem struct {
	TextChunkID string  `json:"text_chunk_id"`
	Content     string  `json:"content"`
	DocumentID  string  `json:"document_id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Rank        float64 `json:"rank"`
}

func (c *Controller) SimpleSearch(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	query := echoCtx.QueryParam("q")
	if query == "" {
		return handleGenericError(echoCtx, echo.NewHTTPError(http.StatusBadRequest, "query parameter 'q' is required"), http.StatusBadRequest)
	}
	nDoc, err := strconv.Atoi(echoCtx.QueryParam("n"))
	if err != nil || nDoc <= 0 {
		nDoc = 100
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
