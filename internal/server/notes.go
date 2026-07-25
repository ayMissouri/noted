package server

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/ayMissouri/noted/internal/storage/db"
)

type vaultJSON struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ChangeSeq int64  `json:"change_seq"`
}

type noteJSON struct {
	ID        string `json:"id"`
	VaultID   string `json:"vault_id"`
	Name      string `json:"name"`
	Body      string `json:"body"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ChangeSeq int64  `json:"change_seq"`
}

type noteListItemJSON struct {
	ID        string `json:"id"`
	VaultID   string `json:"vault_id"`
	Name      string `json:"name"`
	Version   int64  `json:"version"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	ChangeSeq int64  `json:"change_seq"`
}

func toNoteJSON(n db.Note) noteJSON {
	return noteJSON{
		ID: n.ID, VaultID: n.VaultID, Name: n.Name, Body: n.Body, Version: n.Version,
		CreatedAt: n.CreatedAt, UpdatedAt: n.UpdatedAt, ChangeSeq: n.ChangeSeq,
	}
}

func (s *Server) handleListVaults(c *echo.Context) error {
	vaults, err := s.notes.Vaults(c.Request().Context())
	if err != nil {
		return err
	}
	out := make([]vaultJSON, 0, len(vaults))
	for _, v := range vaults {
		out = append(out, vaultJSON{
			ID: v.ID, Name: v.Name, CreatedAt: v.CreatedAt, UpdatedAt: v.UpdatedAt, ChangeSeq: v.ChangeSeq,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"vaults": out})
}

func (s *Server) handleListNotes(c *echo.Context) error {
	rows, err := s.notes.List(c.Request().Context(), c.Param("vaultID"))
	if err != nil {
		return err
	}
	out := make([]noteListItemJSON, 0, len(rows))
	for _, r := range rows {
		out = append(out, noteListItemJSON{
			ID: r.ID, VaultID: r.VaultID, Name: r.Name, Version: r.Version,
			CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt, ChangeSeq: r.ChangeSeq,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"notes": out})
}

type createNoteRequest struct {
	Name string `json:"name"`
	Body string `json:"body"`
}

func (s *Server) handleCreateNote(c *echo.Context) error {
	var req createNoteRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	note, err := s.notes.Create(c.Request().Context(), c.Param("vaultID"), req.Name, req.Body, actorFrom(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusCreated, toNoteJSON(note))
}

func (s *Server) handleGetNote(c *echo.Context) error {
	note, err := s.notes.Get(c.Request().Context(), c.Param("id"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toNoteJSON(note))
}

type updateNoteRequest struct {
	Body        *string `json:"body"`
	BaseVersion int64   `json:"base_version"`
}

func (s *Server) handleUpdateNote(c *echo.Context) error {
	var req updateNoteRequest
	if err := c.Bind(&req); err != nil {
		return err
	}
	if req.Body == nil {
		return echo.NewHTTPError(http.StatusBadRequest, "body is required")
	}
	if req.BaseVersion < 1 {
		return echo.NewHTTPError(http.StatusBadRequest, "base_version is required and must be at least 1")
	}
	note, err := s.notes.Update(c.Request().Context(), c.Param("id"), req.BaseVersion, *req.Body, actorFrom(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, toNoteJSON(note))
}
