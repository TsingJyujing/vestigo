package utils

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

// CreateBearerTokenMiddleware creates a middleware that validates Bearer tokens
func CreateBearerTokenMiddleware(validTokens []string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			// Get Authorization header
			auth := c.Request().Header.Get("Authorization")
			if auth == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "missing authorization header")
			}

			// Check if it's a Bearer token
			const bearerPrefix = "Bearer "
			if len(auth) < len(bearerPrefix) || auth[:len(bearerPrefix)] != bearerPrefix {
				return echo.NewHTTPError(http.StatusUnauthorized, "invalid authorization header format")
			}

			// Extract token
			token := auth[len(bearerPrefix):]

			// Validate token
			for _, validToken := range validTokens {
				if token == validToken {
					return next(c)
				}
			}

			return echo.NewHTTPError(http.StatusUnauthorized, "invalid token")
		}
	}
}
