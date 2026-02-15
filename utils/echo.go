package utils

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v5"
)

const (
	RowNotFoundMessage = "sql: no rows in result set"
)

// EchoHandleSQLError return http response by error return from sql
func EchoHandleSQLError(echoCtx *echo.Context, err error) error {
	if err.Error() == RowNotFoundMessage {
		return (*echoCtx).JSON(http.StatusNotFound, map[string]string{"status": "not found"})
	} // TODO handle more SQL errors, e.g. constraint violation for duplicate document ID
	logger.WithError(err).Error("Unknown SQL error")
	return (*echoCtx).JSON(http.StatusInternalServerError, map[string]string{"status": err.Error()})
}

func EchoHandleGenericError(echoCtx *echo.Context, err error, status int) error {
	logger.WithError(err).WithField("status", status).Error("Error handling request")
	return (*echoCtx).JSON(status, map[string]string{"status": err.Error()})
}

func EchoHandleInternalError(echoCtx *echo.Context, err error) error {
	return EchoHandleGenericError(echoCtx, err, http.StatusInternalServerError)
}

func EchoJsonResponse(echoCtx *echo.Context, data any, status int) error {
	jsonString, err := json.Marshal(data)
	if err != nil {
		return EchoHandleInternalError(echoCtx, err)
	}
	return (*echoCtx).JSONBlob(status, jsonString)
}
