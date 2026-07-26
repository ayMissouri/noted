package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ayMissouri/noted/internal/buildinfo"
)

func (s *Server) handleServerInfo(c *echo.Context) error {
	info := buildinfo.Get()
	return c.JSON(http.StatusOK, map[string]any{
		"name":        s.cfg.ServerName,
		"version":     info.Version,
		"commit":      info.Commit,
		"api_version": "v1",
		"features":    map[string]bool{},
	})
}
