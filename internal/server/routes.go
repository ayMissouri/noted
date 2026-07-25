package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) routes() {
	s.echo.GET("/healthz", s.handleHealthz)

	api := s.echo.Group("/api/v1")
	api.GET("/vaults", s.handleListVaults)
	api.GET("/vaults/:vaultID/notes", s.handleListNotes)
	api.POST("/vaults/:vaultID/notes", s.handleCreateNote)
	api.GET("/notes/:id", s.handleGetNote)
	api.PUT("/notes/:id", s.handleUpdateNote)
	api.GET("/notes/:id/html", s.handleNoteHTML)
	api.POST("/render", s.handleRender)
}

func (s *Server) handleHealthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
