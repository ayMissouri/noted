package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) handleListDevices(c *echo.Context) error {
	id, ok := currentIdentity(c)
	if !ok {
		return errAuthRequired
	}
	tokens, err := s.auth.Tokens(c.Request().Context(), id.User.ID)
	if err != nil {
		return err
	}
	out := make([]tokenJSON, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, tokenJSON{
			ID: t.ID, Name: t.Name, Kind: t.Kind,
			CreatedAt: t.CreatedAt, LastSeenAt: t.LastSeenAt,
			Current: t.ID == id.Token.ID,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"devices": out})
}

func (s *Server) handleRevokeDevice(c *echo.Context) error {
	id, ok := currentIdentity(c)
	if !ok {
		return errAuthRequired
	}
	if err := s.auth.RevokeToken(c.Request().Context(), id.User.ID, c.Param("id")); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
