package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func (s *Server) handleListUsers(c *echo.Context) error {
	users, err := s.auth.Users(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]userJSON, 0, len(users))
	for _, u := range users {
		out = append(out, toUserJSON(u))
	}
	return c.JSON(http.StatusOK, map[string]any{"users": out})
}

type createUserRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
	IsAdmin  bool   `json:"is_admin"`
}

func (s *Server) handleCreateUser(c *echo.Context) error {
	var req createUserRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	user, err := s.auth.CreateUser(c.Request().Context(), req.Username, req.Email, req.Password, req.IsAdmin)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toUserJSON(user))
}
