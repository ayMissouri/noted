package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ayMissouri/noted/internal/storage/db"
)

type userJSON struct {
	ID        string  `json:"id"`
	Username  string  `json:"username"`
	Email     *string `json:"email"`
	IsAdmin   bool    `json:"is_admin"`
	CreatedAt string  `json:"created_at"`
}

func toUserJSON(u db.User) userJSON {
	return userJSON{ID: u.ID, Username: u.Username, Email: u.Email, IsAdmin: u.IsAdmin == 1, CreatedAt: u.CreatedAt}
}

func (s *Server) handleSetupStatus(c *echo.Context) error {
	n, err := s.auth.UserCount(c.Request().Context())
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, map[string]bool{"needs_setup": n == 0})
}

type setupRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (s *Server) handleSetup(c *echo.Context) error {
	var req setupRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	user, err := s.auth.CreateFirstAdmin(c.Request().Context(), req.Username, req.Email, req.Password)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toUserJSON(user))
}
