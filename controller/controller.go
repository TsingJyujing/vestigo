package controller

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/sirupsen/logrus"
	"github.com/tsingjyujing/vestigo/controller/dao"
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
	// Context used by all APIs
	db dao.Queries
}

func New(db dao.Queries) *Controller {
	return &Controller{db: db}
}

// Close closes all resources held by the controller
func (c *Controller) Close() error {
	// TODO close search indexes
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

func (c *Controller) GetDatasource(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	datasourceId := echoCtx.Param("datasource_id")
	ds, err := c.db.GetDatasource(ctx, datasourceId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, ds)
}

func (c *Controller) NewDatasource(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	param := dao.NewDatasourceParams{}
	if err := echoCtx.Bind(&param); err != nil {
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"status": err.Error()})
	}
	_, err := c.db.NewDatasource(ctx, param)
	if err != nil {
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
	}
	return echoCtx.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

func (c *Controller) DeleteDatasource(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	datasourceId := echoCtx.Param("datasource_id")
	err := c.db.DeleteDatasource(ctx, datasourceId)
	if err != nil {
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}

func (c *Controller) NewDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	datasourceId := echoCtx.Param("datasource_id")
	// TODO add text chunks as well
	param := dao.NewDocumentParams{}
	if err := echoCtx.Bind(&param); err != nil {
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"status": err.Error()})
	}
	param.DatasourceID = datasourceId
	_, err := c.db.NewDocument(ctx, param)
	if err != nil {
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
	}
	return echoCtx.JSON(http.StatusCreated, map[string]string{"status": "ok"})
}

func (c *Controller) GetDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	doc, err := c.db.GetDocument(ctx, docId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, doc) // TODO add all text chunks by config
}

func (c *Controller) DeleteDocument(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	docId := echoCtx.Param("doc_id")
	// TODO delete from indexes for each index
	err := c.db.DeleteDocument(ctx, docId)
	if err != nil {
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
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
		return echoCtx.JSON(http.StatusBadRequest, map[string]string{"status": err.Error()})
	}
	requestParam := dao.NewTextChunkParams{
		DocumentID: docId,
		Content:    param.Content,
		ID:         "", // TODO generate UUID
	}
	newText, err := c.db.NewTextChunk(ctx, requestParam)
	// TODO add to indexes
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusCreated, newText)
}

func (c *Controller) GetTextChunk(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	textId := echoCtx.Param("text_id")
	textChunk, err := c.db.GetTextChunk(ctx, textId)
	if err != nil {
		return handleSQLError(echoCtx, err)
	}
	return echoCtx.JSON(http.StatusOK, textChunk)
}

func (c *Controller) DeleteTextChunk(echoCtx echo.Context) error {
	ctx := echoCtx.Request().Context()
	textId := echoCtx.Param("text_id")
	err := c.db.DeleteTextChunk(ctx, textId)
	// TODO delete from indexes
	if err != nil {
		return echoCtx.JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
	}
	return echoCtx.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
