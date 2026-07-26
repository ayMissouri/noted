package server

import (
	_ "embed"
	"net/http"

	"github.com/labstack/echo/v5"
)

//go:embed openapi.yaml
var openapiSpec []byte

func (s *Server) handleOpenAPI(c *echo.Context) error {
	return c.Blob(http.StatusOK, "application/yaml", openapiSpec)
}
