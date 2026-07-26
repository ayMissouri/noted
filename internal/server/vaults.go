package server

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

type vaultRequest struct {
	Name string `json:"name"`
}

func (s *Server) handleCreateVault(c *echo.Context) error {
	var req vaultRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	vault, err := s.notes.CreateVault(c.Request().Context(), req.Name, actorFrom(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, vaultJSON{
		ID: vault.ID, Name: vault.Name, CreatedAt: vault.CreatedAt, UpdatedAt: vault.UpdatedAt, ChangeSeq: vault.ChangeSeq,
	})
}

func (s *Server) handleRenameVault(c *echo.Context) error {
	var req vaultRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	vault, err := s.notes.RenameVault(c.Request().Context(), c.Param("vaultID"), req.Name, actorFrom(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, vaultJSON{
		ID: vault.ID, Name: vault.Name, CreatedAt: vault.CreatedAt, UpdatedAt: vault.UpdatedAt, ChangeSeq: vault.ChangeSeq,
	})
}

func (s *Server) handleDeleteVault(c *echo.Context) error {
	if err := s.notes.DeleteVault(c.Request().Context(), c.Param("vaultID"), actorFrom(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}
