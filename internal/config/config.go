package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type Config struct {
	DataDir string
	// ListenAddr is the host:port the HTTP server binds to.
	ListenAddr string
	// BaseURL is the public URL at which clients reach this server.
	BaseURL   *url.URL
	LogLevel  slog.Level
	LogFormat string
	// AutoMigrate applies pending migrations on boot.
	AutoMigrate bool
	// CORSOrigins lists urls allowed to call the API.
	CORSOrigins []string
	ServerName  string
}

const (
	defaultDataDir    = "data"
	defaultListenAddr = "127.0.0.1:6683"
	defaultLogFormat  = "text"
)

func Load(getenv func(string) string) (*Config, error) {
	cfg := &Config{
		DataDir:     defaultDataDir,
		ListenAddr:  defaultListenAddr,
		LogLevel:    slog.LevelInfo,
		LogFormat:   defaultLogFormat,
		AutoMigrate: true,
		ServerName:  "noted",
	}
	var errs []error
	fail := func(variable, got, want string) {
		errs = append(errs, fmt.Errorf("%s: invalid value %q, want %s", variable, got, want))
	}

	if v := getenv("NOTED_DATA_DIR"); v != "" {
		cfg.DataDir = v
	}

	if v := getenv("NOTED_LISTEN_ADDR"); v != "" {
		if err := validateListenAddr(v); err != nil {
			fail("NOTED_LISTEN_ADDR", v, `a host:port such as ":6683" or "127.0.0.1:6683"`)
		} else {
			cfg.ListenAddr = v
		}
	} else if port := getenv("PORT"); port != "" {
		if !validPort(port) {
			fail("PORT", port, "a port number between 1 and 65535")
		} else {
			cfg.ListenAddr = ":" + port
		}
	}

	if v := getenv("NOTED_BASE_URL"); v != "" {
		u, err := parseBaseURL(v)
		if err != nil {
			fail("NOTED_BASE_URL", v, err.Error())
		} else {
			cfg.BaseURL = u
		}
	}

	if v := getenv("NOTED_LOG_LEVEL"); v != "" {
		level, ok := parseLogLevel(v)
		if !ok {
			fail("NOTED_LOG_LEVEL", v, "one of debug, info, warn, error")
		} else {
			cfg.LogLevel = level
		}
	}

	if v := getenv("NOTED_LOG_FORMAT"); v != "" {
		switch strings.ToLower(v) {
		case "text", "json":
			cfg.LogFormat = strings.ToLower(v)
		default:
			fail("NOTED_LOG_FORMAT", v, `"text" or "json"`)
		}
	}

	if v := strings.TrimSpace(getenv("NOTED_SERVER_NAME")); v != "" {
		if len(v) > 100 {
			fail("NOTED_SERVER_NAME", v, "a name of at most 100 characters")
		} else {
			cfg.ServerName = v
		}
	}

	if v := getenv("NOTED_CORS_ORIGINS"); v != "" {
		for _, raw := range strings.Split(v, ",") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			if raw == "*" {
				fail("NOTED_CORS_ORIGINS", raw, "an explicit comma-separated origin list; wildcards are not allowed")
				continue
			}
			u, err := parseBaseURL(raw)
			if err != nil {
				fail("NOTED_CORS_ORIGINS", raw, err.Error())
				continue
			}
			cfg.CORSOrigins = append(cfg.CORSOrigins, u.String())
		}
	}

	if v := getenv("NOTED_AUTO_MIGRATE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			fail("NOTED_AUTO_MIGRATE", v, "true or false")
		} else {
			cfg.AutoMigrate = b
		}
	}

	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return cfg, nil
}

func validateListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return err
	}
	_ = host
	if !validPort(port) {
		return fmt.Errorf("port %q out of range", port)
	}
	return nil
}

func validPort(s string) bool {
	n, err := strconv.Atoi(s)
	return err == nil && n >= 1 && n <= 65535
}

// parseBaseURL accepts only scheme://host[:port] because share links and OAuth redirects are built by joining paths onto it.
func parseBaseURL(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, errors.New(`an absolute URL such as "https://notes.example.com"`)
	}
	switch {
	case u.Scheme != "http" && u.Scheme != "https":
		return nil, errors.New(`a URL starting with http:// or https://`)
	case u.Host == "":
		return nil, errors.New(`an absolute URL with a host, such as "https://notes.example.com"`)
	case u.Path != "" && u.Path != "/":
		return nil, errors.New(`a URL with no path; serving under a path prefix is not supported`)
	case u.RawQuery != "" || u.Fragment != "":
		return nil, errors.New(`a URL with no query or fragment`)
	}
	u.Path = ""
	return u, nil
}

func parseLogLevel(s string) (slog.Level, bool) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, true
	case "info":
		return slog.LevelInfo, true
	case "warn":
		return slog.LevelWarn, true
	case "error":
		return slog.LevelError, true
	}
	return 0, false
}
