package server

import (
	"io/fs"
	"net/http"
	"path"
	"strings"

	"github.com/labstack/echo/v5"
)

// spaHandler serves the embedded client.
func (s *Server) spaHandler() echo.HandlerFunc {
	fileServer := http.FileServerFS(s.assets)
	return func(c *echo.Context) error {
		p := c.Request().URL.Path
		if strings.HasPrefix(p, "/api/") || p == "/healthz" {
			return echo.ErrNotFound
		}
		name := strings.TrimPrefix(path.Clean(p), "/")
		if name == "" {
			name = "index.html"
		}
		if _, err := fs.Stat(s.assets, name); err != nil {
			c.Request().URL.Path = "/"
			name = "index.html"
		}
		if strings.HasPrefix(name, "assets/") {
			c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			c.Response().Header().Set("Cache-Control", "no-cache")
		}
		fileServer.ServeHTTP(c.Response(), c.Request())
		return nil
	}
}
