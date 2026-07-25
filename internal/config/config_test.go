package config

import (
	"log/slog"
	"strings"
	"testing"
)

func getenvFrom(env map[string]string) func(string) string {
	return func(key string) string { return env[key] }
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := Load(getenvFrom(nil))
	if err != nil {
		t.Fatalf("Load with empty environment: %v", err)
	}
	if cfg.DataDir != "data" {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, "data")
	}
	if cfg.ListenAddr != ":6683" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":6683")
	}
	if cfg.BaseURL != nil {
		t.Errorf("BaseURL = %v, want nil", cfg.BaseURL)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("LogFormat = %q, want %q", cfg.LogFormat, "text")
	}
	if !cfg.AutoMigrate {
		t.Error("AutoMigrate = false, want true")
	}
}

func TestLoadValidValues(t *testing.T) {
	tests := []struct {
		name  string
		env   map[string]string
		check func(t *testing.T, cfg *Config)
	}{
		{
			name: "explicit listen addr",
			env:  map[string]string{"NOTED_LISTEN_ADDR": "127.0.0.1:9000"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.ListenAddr != "127.0.0.1:9000" {
					t.Errorf("ListenAddr = %q, want 127.0.0.1:9000", cfg.ListenAddr)
				}
			},
		},
		{
			name: "PORT fallback",
			env:  map[string]string{"PORT": "8080"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.ListenAddr != ":8080" {
					t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
				}
			},
		},
		{
			name: "NOTED_LISTEN_ADDR wins over PORT",
			env:  map[string]string{"NOTED_LISTEN_ADDR": ":7000", "PORT": "8080"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.ListenAddr != ":7000" {
					t.Errorf("ListenAddr = %q, want :7000", cfg.ListenAddr)
				}
			},
		},
		{
			name: "base URL keeps scheme and host",
			env:  map[string]string{"NOTED_BASE_URL": "https://notes.example.com"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.BaseURL == nil || cfg.BaseURL.String() != "https://notes.example.com" {
					t.Errorf("BaseURL = %v, want https://notes.example.com", cfg.BaseURL)
				}
			},
		},
		{
			name: "base URL trailing slash stripped",
			env:  map[string]string{"NOTED_BASE_URL": "https://notes.example.com/"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.BaseURL == nil || cfg.BaseURL.String() != "https://notes.example.com" {
					t.Errorf("BaseURL = %v, want https://notes.example.com", cfg.BaseURL)
				}
			},
		},
		{
			name: "auto migrate off",
			env:  map[string]string{"NOTED_AUTO_MIGRATE": "false"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.AutoMigrate {
					t.Error("AutoMigrate = true, want false")
				}
			},
		},
		{
			name: "log settings",
			env:  map[string]string{"NOTED_LOG_LEVEL": "DEBUG", "NOTED_LOG_FORMAT": "JSON"},
			check: func(t *testing.T, cfg *Config) {
				if cfg.LogLevel != slog.LevelDebug {
					t.Errorf("LogLevel = %v, want debug", cfg.LogLevel)
				}
				if cfg.LogFormat != "json" {
					t.Errorf("LogFormat = %q, want json", cfg.LogFormat)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := Load(getenvFrom(tt.env))
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			tt.check(t, cfg)
		})
	}
}

func TestLoadInvalidValues(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantIn  string
	}{
		{
			name:   "bad log level names the variable",
			env:    map[string]string{"NOTED_LOG_LEVEL": "verbose"},
			wantIn: "NOTED_LOG_LEVEL",
		},
		{
			name:   "bad log format",
			env:    map[string]string{"NOTED_LOG_FORMAT": "xml"},
			wantIn: "NOTED_LOG_FORMAT",
		},
		{
			name:   "listen addr without port",
			env:    map[string]string{"NOTED_LISTEN_ADDR": "localhost"},
			wantIn: "NOTED_LISTEN_ADDR",
		},
		{
			name:   "listen addr port out of range",
			env:    map[string]string{"NOTED_LISTEN_ADDR": ":70000"},
			wantIn: "NOTED_LISTEN_ADDR",
		},
		{
			name:   "bad PORT",
			env:    map[string]string{"PORT": "web"},
			wantIn: "PORT",
		},
		{
			name:   "bad auto migrate",
			env:    map[string]string{"NOTED_AUTO_MIGRATE": "maybe"},
			wantIn: "NOTED_AUTO_MIGRATE",
		},
		{
			name:   "base URL with path",
			env:    map[string]string{"NOTED_BASE_URL": "https://example.com/notes"},
			wantIn: "path prefix",
		},
		{
			name:   "base URL without scheme",
			env:    map[string]string{"NOTED_BASE_URL": "notes.example.com"},
			wantIn: "NOTED_BASE_URL",
		},
		{
			name:   "base URL with bad scheme",
			env:    map[string]string{"NOTED_BASE_URL": "ftp://example.com"},
			wantIn: "http://",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Load(getenvFrom(tt.env))
			if err == nil {
				t.Fatal("Load succeeded, want error")
			}
			if !strings.Contains(err.Error(), tt.wantIn) {
				t.Errorf("error %q does not mention %q", err, tt.wantIn)
			}
		})
	}
}

func TestLoadReportsAllErrorsTogether(t *testing.T) {
	_, err := Load(getenvFrom(map[string]string{
		"NOTED_LOG_LEVEL":  "verbose",
		"NOTED_LOG_FORMAT": "xml",
	}))
	if err == nil {
		t.Fatal("Load succeeded, want error")
	}
	for _, want := range []string{"NOTED_LOG_LEVEL", "NOTED_LOG_FORMAT"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}
