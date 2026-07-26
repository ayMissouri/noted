package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) routes() {
	s.echo.GET("/healthz", s.handleHealthz)

	api := s.echo.Group("/api/v1")
	authLimit := authRateLimiter()
	api.GET("/server", s.handleServerInfo)
	api.GET("/openapi.yaml", s.handleOpenAPI)
	api.GET("/setup", s.handleSetupStatus)
	api.POST("/setup", s.handleSetup, authLimit)
	api.POST("/login", s.handleLogin, authLimit)

	authed := api.Group("", s.requireAuth())
	authed.GET("/devices", s.handleListDevices)
	authed.DELETE("/devices/:id", s.handleRevokeDevice)
	authed.GET("/vaults", s.handleListVaults)
	authed.POST("/vaults", s.handleCreateVault)
	authed.PATCH("/vaults/:vaultID", s.handleRenameVault)
	authed.DELETE("/vaults/:vaultID", s.handleDeleteVault)
	authed.GET("/vaults/:vaultID/notes", s.handleListNotes)
	authed.POST("/vaults/:vaultID/notes", s.handleCreateNote)
	authed.GET("/notes/:id", s.handleGetNote)
	authed.PUT("/notes/:id", s.handleUpdateNote)
	authed.DELETE("/notes/:id", s.handleTrashNote)
	authed.POST("/notes/:id/restore", s.handleRestoreNote)
	authed.GET("/notes/:id/html", s.handleNoteHTML)
	authed.POST("/render", s.handleRender)

	admin := authed.Group("", s.requireAdmin())
	admin.GET("/users", s.handleListUsers)
	admin.POST("/users", s.handleCreateUser)

	s.echo.RouteNotFound("/*", s.spaHandler())
}

func (s *Server) handleHealthz(c *echo.Context) error {
	return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
}
