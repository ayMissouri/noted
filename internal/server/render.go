package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ayMissouri/noted/internal/markdown"
)

type renderRequest struct {
	Markdown *string `json:"markdown"`
	VaultID  string  `json:"vault_id"`
}

func (s *Server) handleRender(c *echo.Context) error {
	var req renderRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if req.Markdown == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "markdown is required")
	}
	ctx := c.Request().Context()

	var resolver markdown.Resolver
	if req.VaultID != "" {
		if _, _, err := s.notes.List(ctx, req.VaultID, actorFrom(c), 0); err != nil {
			return err
		}
		resolver = s.notes.Resolver(ctx, req.VaultID)
	}

	html, err := s.render.Render([]byte(*req.Markdown), resolver)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"html": string(html)})
}

func (s *Server) handleNoteHTML(c *echo.Context) error {
	ctx := c.Request().Context()
	note, err := s.notes.Get(ctx, c.Param("id"), actorFrom(c))
	if err != nil {
		return err
	}
	html, err := s.render.Render([]byte(note.Body), s.notes.Resolver(ctx, note.VaultID))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"html": string(html), "version": note.Version})
}
