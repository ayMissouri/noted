package server

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ayMissouri/noted/internal/config"
	"github.com/ayMissouri/noted/internal/markdown"
	"github.com/ayMissouri/noted/internal/notes"
)

const maxBodyBytes = 10 << 20

type Server struct {
	echo   *echo.Echo
	cfg    *config.Config
	logger *slog.Logger
	notes  *notes.Service
	render *markdown.Renderer
	assets fs.FS
}

func New(cfg *config.Config, logger *slog.Logger, notesSvc *notes.Service, renderer *markdown.Renderer, assets fs.FS) *Server {
	e := echo.New()
	s := &Server{echo: e, cfg: cfg, logger: logger, notes: notesSvc, render: renderer, assets: assets}
	e.HTTPErrorHandler = s.handleError
	e.Use(middleware.RequestID())
	e.Use(s.requestLogger())
	e.Use(middleware.Recover())
	e.Use(middleware.BodyLimit(maxBodyBytes))
	s.routes()
	return s
}

// Handler exposes the server for httptest.
func (s *Server) Handler() http.Handler { return s.echo }

func (s *Server) Run(ctx context.Context) error {
	s.logger.Info("server listening", "addr", s.cfg.ListenAddr)
	sc := echo.StartConfig{Address: s.cfg.ListenAddr, GracefulTimeout: 10 * time.Second}
	err := sc.Start(ctx, s.echo)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http server: %w", err)
	}
	return nil
}

var domainErrors = []struct {
	err    error
	status int
	code   string
}{
	{notes.ErrNotFound, http.StatusNotFound, "not_found"},
	{notes.ErrVaultNotFound, http.StatusNotFound, "vault_not_found"},
	{notes.ErrVersionConflict, http.StatusConflict, "version_conflict"},
	{notes.ErrNameTaken, http.StatusConflict, "name_taken"},
	{notes.ErrInvalidName, http.StatusUnprocessableEntity, "invalid_name"},
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (s *Server) handleError(c *echo.Context, err error) {
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}
	status := http.StatusInternalServerError
	code := "internal"
	msg := "internal server error"

	matched := false
	for _, d := range domainErrors {
		if errors.Is(err, d.err) {
			status, code, msg = d.status, d.code, err.Error()
			matched = true
			break
		}
	}
	if !matched {
		var sc echo.HTTPStatusCoder
		if errors.As(err, &sc) {
			status = sc.StatusCode()
			code = codeForStatus(status)
			msg = http.StatusText(status)
			var he *echo.HTTPError
			if errors.As(err, &he) && he.Message != "" {
				msg = he.Message
			}
		}
	}
	if status >= 500 {
		code, msg = "internal", "internal server error"
		s.logger.Error("request failed", "error", err, "request_id", requestID(c))
	}
	if werr := c.JSON(status, map[string]errorBody{"error": {Code: code, Message: msg}}); werr != nil {
		s.logger.Error("writing error response failed", "error", werr, "request_id", requestID(c))
	}
}

func codeForStatus(status int) string {
	return strings.ToLower(strings.ReplaceAll(http.StatusText(status), " ", "_"))
}

func (s *Server) requestLogger() echo.MiddlewareFunc {
	return middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogStatus:    true,
		LogMethod:    true,
		LogURI:       true,
		LogLatency:   true,
		LogRequestID: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			level := slog.LevelInfo
			if v.Error != nil || v.Status >= 500 {
				level = slog.LevelError
			}
			s.logger.Log(c.Request().Context(), level, "request",
				"method", v.Method, "uri", v.URI, "status", v.Status,
				"latency_ms", v.Latency.Milliseconds(), "request_id", v.RequestID)
			return nil
		},
	})
}

func requestID(c *echo.Context) string {
	return c.Response().Header().Get(echo.HeaderXRequestID)
}
