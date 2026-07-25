package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"

	"github.com/ahmedmissouri/noted/internal/config"
)

type Server struct {
	echo   *echo.Echo
	cfg    *config.Config
	logger *slog.Logger
}

func New(cfg *config.Config, logger *slog.Logger) *Server {
	e := echo.New()
	s := &Server{echo: e, cfg: cfg, logger: logger}
	e.HTTPErrorHandler = s.handleError
	e.Use(middleware.RequestID())
	e.Use(s.requestLogger())
	e.Use(middleware.Recover())
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

func (s *Server) handleError(c *echo.Context, err error) {
	if r, _ := echo.UnwrapResponse(c.Response()); r != nil && r.Committed {
		return
	}
	code := http.StatusInternalServerError
	var sc echo.HTTPStatusCoder
	if errors.As(err, &sc) {
		code = sc.StatusCode()
	}
	msg := http.StatusText(code)
	var he *echo.HTTPError
	if errors.As(err, &he) && he.Message != "" {
		msg = he.Message
	}
	if code >= 500 {
		msg = "internal server error"
		s.logger.Error("request failed", "error", err, "request_id", requestID(c))
	}
	if werr := c.JSON(code, map[string]string{"error": msg}); werr != nil {
		s.logger.Error("writing error response failed", "error", werr, "request_id", requestID(c))
	}
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
