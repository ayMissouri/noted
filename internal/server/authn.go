package server

import (
	"errors"
	"net/http"
	"strings"

	"github.com/labstack/echo/v5"

	"github.com/ayMissouri/noted/internal/auth"
	"github.com/ayMissouri/noted/internal/notes"
	"github.com/ayMissouri/noted/internal/storage/db"
)

var errAuthRequired = errors.New("authentication required")

const identityContextKey = "noted.identity"

type identity struct {
	User  db.User
	Token db.Token
}

func (s *Server) requireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			header := c.Request().Header.Get(echo.HeaderAuthorization)
			secret, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || secret == "" {
				return errAuthRequired
			}
			token, user, err := s.auth.ValidateToken(c.Request().Context(), secret)
			if err != nil {
				return err
			}
			c.Set(identityContextKey, identity{User: user, Token: token})
			return next(c)
		}
	}
}

func currentIdentity(c *echo.Context) (identity, bool) {
	id, ok := c.Get(identityContextKey).(identity)
	return id, ok
}

func actorFrom(c *echo.Context) notes.Actor {
	id, ok := currentIdentity(c)
	if !ok {
		return notes.Actor{Kind: notes.KindUser}
	}
	return notes.Actor{Kind: notes.KindUser, UserID: &id.User.ID, TokenID: &id.Token.ID}
}

type tokenJSON struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Kind       string  `json:"kind"`
	CreatedAt  string  `json:"created_at"`
	LastSeenAt *string `json:"last_seen_at"`
	Current    bool    `json:"current"`
}

type loginRequest struct {
	Username   string `json:"username"`
	Password   string `json:"password"`
	DeviceName string `json:"device_name"`
	Kind       string `json:"kind"`
}

func (s *Server) handleLogin(c *echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	kind := req.Kind
	if kind == "" {
		kind = auth.TokenWeb
	}
	if kind != auth.TokenWeb && kind != auth.TokenCLI {
		return echo.NewHTTPError(http.StatusBadRequest, "kind must be web or cli")
	}
	secret, token, user, err := s.auth.Authenticate(c.Request().Context(), req.Username, req.Password, req.DeviceName, kind)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]any{
		"token": secret,
		"user":  toUserJSON(user),
		"device": tokenJSON{
			ID: token.ID, Name: token.Name, Kind: token.Kind,
			CreatedAt: token.CreatedAt, LastSeenAt: token.LastSeenAt,
			Current: true,
		},
	})
}
