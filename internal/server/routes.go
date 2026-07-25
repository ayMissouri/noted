package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) routes() {
	s.echo.GET("/healthz", s.handleHealthz)
}

func (s *Server) handleHealthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
