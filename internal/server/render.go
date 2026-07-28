package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type renderRequest struct {
	Markdown *string `json:"markdown"`
}

func (s *Server) handleRender(c *echo.Context) error {
	var req renderRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if req.Markdown == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "markdown is required")
	}
	html, err := s.render.Render([]byte(*req.Markdown), nil)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]string{"html": string(html)})
}

func (s *Server) handleNoteHTML(c *echo.Context) error {
	note, err := s.notes.Get(c.Request().Context(), c.Param("id"), actorFrom(c))
	if err != nil {
		return err
	}
	html, err := s.render.Render([]byte(note.Body), nil)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{"html": string(html), "version": note.Version})
}
