package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) routes() {
	s.echo.GET("/healthz", s.handleHealthz)

	api := s.echo.Group("/api/v1")
	api.GET("/setup", s.handleSetupStatus)
	api.POST("/setup", s.handleSetup)
	api.POST("/login", s.handleLogin)

	authed := api.Group("", s.requireAuth())
	authed.GET("/devices", s.handleListDevices)
	authed.DELETE("/devices/:id", s.handleRevokeDevice)
	authed.GET("/vaults", s.handleListVaults)
	authed.GET("/vaults/:vaultID/notes", s.handleListNotes)
	authed.POST("/vaults/:vaultID/notes", s.handleCreateNote)
	authed.GET("/notes/:id", s.handleGetNote)
	authed.PUT("/notes/:id", s.handleUpdateNote)
	authed.GET("/notes/:id/html", s.handleNoteHTML)
	authed.POST("/render", s.handleRender)

	s.echo.RouteNotFound("/*", s.spaHandler())
}

func (s *Server) handleHealthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
