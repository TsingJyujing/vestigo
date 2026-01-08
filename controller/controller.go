package controller

import (
	"context"
	"database/sql"
	_ "embed"
	"net/http"
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

func (c *Controller) GetDatasource(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	datasourceId := echoCtx.Param("datasource_id")
	ds, err := c.queries.GetDatasource(ctx, datasourceId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, ds)
}

func (c *Controller) NewDatasource(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	param := dao.NewDatasourceParams{}
	if err := echoCtx.Bind(&param); err != nil {
		return handleGenericError(echoCtx, err, http.StatusBadRequest)
	}
	_, err := c.queries.NewDatasource(ctx, param)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

func (c *Controller) DeleteDatasource(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	datasourceId := echoCtx.Param("datasource_id")
	err := c.queries.DeleteDatasource(ctx, datasourceId)
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

type NewDocumentParams struct {
	ID          string
	Title       string
	Description string
	Texts       []string
}

func (c *Controller) NewDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	datasourceId := echoCtx.Param("datasource_id")

	param := NewDocumentParams{}
	if err := echoCtx.Bind(&param); err != nil {
		return handleGenericError(echoCtx, err, http.StatusBadRequest)
	}
	insertCount, err := utils.WithTx(
		ctx,
		c.db,
		nil,
		func(tx *sql.Tx) (int, error) {
			err := dao.New(tx).NewDocument(ctx, dao.NewDocumentParams{
				ID:           param.ID,
				DatasourceID: datasourceId,
				Title:        param.Title,
				Description:  sql.NullString{String: param.Description, Valid: param.Description != ""},
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

func (c *Controller) DeleteDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	// TODO delete from indexes for each index
	err := c.queries.DeleteDocument(ctx, docId)
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
	// Add to BM25 index FTS5 table
	res, err := c.db.ExecContext(ctx, "INSERT INTO text_chunk_fts (rowid, seg_content) VALUES (?, ?)", newText.ID, newText.SegContent)
	if err != nil {
		return nil, err
	}
	rowId, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	logger.WithField("rowId", rowId).Info("insert into text_chunk_fts")
	// TODO add to indexes as well
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
	err := c.queries.DeleteTextChunk(ctx, textId)
	// TODO delete from indexes
	if err != nil {
		return handleInternalError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
