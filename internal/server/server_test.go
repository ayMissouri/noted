package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"

	"github.com/ahmedmissouri/noted/internal/config"
	"github.com/ahmedmissouri/noted/internal/notes"
	"github.com/ahmedmissouri/noted/internal/storage"
)

func testServer(t *testing.T) (*Server, string) {
	t.Helper()
	sqldb, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if _, err := storage.Migrate(context.Background(), sqldb, storage.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	svc := notes.NewService(sqldb)
	vault, err := svc.EnsureDefaultVault(context.Background())
	if err != nil {
		t.Fatalf("EnsureDefaultVault: %v", err)
	}
	cfg, err := config.Load(func(string) string { return "" })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	return New(cfg, logger, svc), vault.ID
}

func do(t *testing.T, s *Server, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, "application/json")
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	return rec
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	env := decode[map[string]errorBody](t, rec)
	return env["error"].Code
}

func TestHealthz(t *testing.T) {
	s, _ := testServer(t)
	rec := do(t, s, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(echo.HeaderXRequestID) == "" {
		t.Error("response has no request id header")
	}
}

func TestUnknownRouteIsJSON404(t *testing.T) {
	s, _ := testServer(t)
	rec := do(t, s, http.MethodGet, "/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", rec.Code)
	}
	if got := errCode(t, rec); got != "not_found" {
		t.Errorf("code = %q, want not_found", got)
	}
}

func TestPanicIsRecoveredAs500(t *testing.T) {
	s, _ := testServer(t)
	s.echo.GET("/boom", func(c *echo.Context) error { panic("boom") })
	rec := do(t, s, http.MethodGet, "/boom", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body %q leaks the panic value", rec.Body.String())
	}
}

func TestListVaults(t *testing.T) {
	s, vaultID := testServer(t)
	rec := do(t, s, http.MethodGet, "/api/v1/vaults", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	out := decode[map[string][]vaultJSON](t, rec)
	if len(out["vaults"]) != 1 || out["vaults"][0].ID != vaultID {
		t.Errorf("vaults = %+v, want the default vault %s", out["vaults"], vaultID)
	}
}

func TestCreateGetUpdateNote(t *testing.T) {
	s, vaultID := testServer(t)

	rec := do(t, s, http.MethodPost, "/api/v1/vaults/"+vaultID+"/notes", `{"name":"Hello","body":"# Hi\n"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}
	created := decode[noteJSON](t, rec)
	if created.Version != 1 || created.Name != "Hello" {
		t.Errorf("created = %+v, want version 1 name Hello", created)
	}

	rec = do(t, s, http.MethodGet, "/api/v1/notes/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
	if got := decode[noteJSON](t, rec); got.Body != "# Hi\n" {
		t.Errorf("body = %q, want # Hi", got.Body)
	}

	rec = do(t, s, http.MethodPut, "/api/v1/notes/"+created.ID, `{"body":"# Hi v2\n","base_version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := decode[noteJSON](t, rec); got.Version != 2 {
		t.Errorf("version = %d, want 2", got.Version)
	}

	rec = do(t, s, http.MethodGet, "/api/v1/vaults/"+vaultID+"/notes", "")
	out := decode[map[string][]noteListItemJSON](t, rec)
	if len(out["notes"]) != 1 || out["notes"][0].Version != 2 {
		t.Errorf("list = %+v, want one note at version 2", out["notes"])
	}
}

func TestAPIErrors(t *testing.T) {
	s, vaultID := testServer(t)
	if rec := do(t, s, http.MethodPost, "/api/v1/vaults/"+vaultID+"/notes", `{"name":"Dup"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed create failed: %d", rec.Code)
	}

	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
		wantCode   string
	}{
		{"missing note", http.MethodGet, "/api/v1/notes/none", "", 404, "not_found"},
		{"missing vault", http.MethodPost, "/api/v1/vaults/none/notes", `{"name":"x"}`, 404, "vault_not_found"},
		{"invalid name", http.MethodPost, "/api/v1/vaults/" + vaultID + "/notes", `{"name":"a/b"}`, 422, "invalid_name"},
		{"duplicate name", http.MethodPost, "/api/v1/vaults/" + vaultID + "/notes", `{"name":"Dup"}`, 409, "name_taken"},
		{"update missing body", http.MethodPut, "/api/v1/notes/none", `{"base_version":1}`, 400, "bad_request"},
		{"update missing base version", http.MethodPut, "/api/v1/notes/none", `{"body":"x"}`, 400, "bad_request"},
		{"malformed json", http.MethodPost, "/api/v1/vaults/" + vaultID + "/notes", `{`, 400, "bad_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, s, tt.method, tt.path, tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := errCode(t, rec); got != tt.wantCode {
				t.Errorf("code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestStaleUpdateConflicts(t *testing.T) {
	s, vaultID := testServer(t)
	rec := do(t, s, http.MethodPost, "/api/v1/vaults/"+vaultID+"/notes", `{"name":"Contested","body":"v1"}`)
	created := decode[noteJSON](t, rec)
	if rec := do(t, s, http.MethodPut, "/api/v1/notes/"+created.ID, `{"body":"v2","base_version":1}`); rec.Code != http.StatusOK {
		t.Fatalf("first update: %d", rec.Code)
	}
	rec = do(t, s, http.MethodPut, "/api/v1/notes/"+created.ID, `{"body":"lost","base_version":1}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if got := errCode(t, rec); got != "version_conflict" {
		t.Errorf("code = %q, want version_conflict", got)
	}
}
