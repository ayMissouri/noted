package server

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/labstack/echo/v5"

	"github.com/ayMissouri/noted/internal/auth"
	"github.com/ayMissouri/noted/internal/config"
	"github.com/ayMissouri/noted/internal/markdown"
	"github.com/ayMissouri/noted/internal/notes"
	"github.com/ayMissouri/noted/internal/storage"
)

type env struct {
	srv     *Server
	sqldb   *sql.DB
	vaultID string
	adminID string
	bearer  string
}

func newBareEnv(t *testing.T) *env {
	return buildEnv(t, nil)
}

func buildEnv(t *testing.T, vars map[string]string) *env {
	t.Helper()
	sqldb, err := storage.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { sqldb.Close() })
	if _, err := storage.Migrate(context.Background(), sqldb, storage.Migrations()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	renderer := markdown.NewRenderer()
	svc := notes.NewService(sqldb, renderer)
	vault, err := svc.EnsureDefaultVault(context.Background())
	if err != nil {
		t.Fatalf("EnsureDefaultVault: %v", err)
	}
	cfg, err := config.Load(func(key string) string { return vars[key] })
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	logger := slog.New(slog.NewTextHandler(&strings.Builder{}, nil))
	assets := fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<!doctype html><title>noted test</title>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log('app')")},
	}
	srv := New(cfg, logger, svc, auth.NewService(sqldb), renderer, assets)
	return &env{srv: srv, sqldb: sqldb, vaultID: vault.ID}
}

const testPassword = "a long test password"

func newEnv(t *testing.T) *env {
	t.Helper()
	e := newBareEnv(t)
	authSvc := auth.NewService(e.sqldb)
	admin, err := authSvc.CreateFirstAdmin(context.Background(), "admin", "", testPassword)
	if err != nil {
		t.Fatalf("CreateFirstAdmin: %v", err)
	}
	secret, _, _, err := authSvc.Authenticate(context.Background(), "admin", testPassword, "test device", auth.TokenWeb)
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	e.adminID = admin.ID
	e.bearer = secret
	return e
}

func (e *env) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set(echo.HeaderContentType, "application/json")
	}
	if e.bearer != "" {
		req.Header.Set(echo.HeaderAuthorization, "Bearer "+e.bearer)
	}
	rec := httptest.NewRecorder()
	e.srv.Handler().ServeHTTP(rec, req)
	return rec
}

func (e *env) doAnon(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	saved := e.bearer
	e.bearer = ""
	defer func() { e.bearer = saved }()
	return e.do(t, method, path, body)
}

func decode[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	return v
}

type vaultsListResponse struct {
	Vaults []vaultJSON `json:"vaults"`
	Cursor int64       `json:"cursor"`
}

type notesListResponse struct {
	Notes  []noteListItemJSON `json:"notes"`
	Cursor int64              `json:"cursor"`
}

func errCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	env := decode[map[string]errorBody](t, rec)
	return env["error"].Code
}

func TestCORS(t *testing.T) {
	e := buildEnv(t, map[string]string{"NOTED_CORS_ORIGINS": "https://app.example.com"})

	preflight := func(origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodOptions, "/api/v1/vaults", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("Access-Control-Request-Method", http.MethodGet)
		req.Header.Set("Access-Control-Request-Headers", "authorization")
		rec := httptest.NewRecorder()
		e.srv.Handler().ServeHTTP(rec, req)
		return rec
	}

	rec := preflight("https://app.example.com")
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://app.example.com" {
		t.Errorf("allowed origin ACAO = %q, want the origin", got)
	}
	if got := rec.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(strings.ToLower(got), "authorization") {
		t.Errorf("allow headers = %q, want authorization", got)
	}

	if got := preflight("https://evil.example").Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("disallowed origin ACAO = %q, want empty", got)
	}

	bare := newBareEnv(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("Origin", "https://app.example.com")
	rec = httptest.NewRecorder()
	bare.srv.Handler().ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("unconfigured server ACAO = %q, want empty", got)
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newEnv(t)

	var last *httptest.ResponseRecorder
	for range 10 {
		last = e.doAnon(t, http.MethodPost, "/api/v1/login", `{"username":"admin","password":"wrong"}`)
	}
	if last.Code != http.StatusUnauthorized {
		t.Fatalf("attempt 10 = %d, want 401 (still within burst)", last.Code)
	}
	rec := e.doAnon(t, http.MethodPost, "/api/v1/login", `{"username":"admin","password":"wrong"}`)
	if rec.Code != http.StatusTooManyRequests {
		t.Errorf("attempt 11 = %d, want 429", rec.Code)
	}
	if got := errCode(t, rec); got != "too_many_requests" {
		t.Errorf("code = %q, want too_many_requests", got)
	}

	if rec := e.do(t, http.MethodGet, "/api/v1/vaults", ""); rec.Code != http.StatusOK {
		t.Errorf("authenticated API limited too: %d, want 200", rec.Code)
	}
}

func TestSecurityHeaders(t *testing.T) {
	e := newBareEnv(t)
	for _, path := range []string{"/", "/healthz"} {
		rec := e.do(t, http.MethodGet, path, "")
		h := rec.Header()
		if got := h.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s X-Content-Type-Options = %q, want nosniff", path, got)
		}
		if got := h.Get("X-Frame-Options"); got != "DENY" {
			t.Errorf("%s X-Frame-Options = %q, want DENY", path, got)
		}
		if got := h.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s Referrer-Policy = %q, want no-referrer", path, got)
		}
		csp := h.Get("Content-Security-Policy")
		if !strings.Contains(csp, "script-src 'self'") {
			t.Errorf("%s CSP = %q, want script-src 'self'", path, csp)
		}
		if strings.Contains(csp, "script-src 'self' 'unsafe-inline'") {
			t.Errorf("%s CSP allows inline scripts: %q", path, csp)
		}
		if !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("%s CSP = %q, want frame-ancestors 'none'", path, csp)
		}
	}
}

func TestServerInfo(t *testing.T) {
	e := newBareEnv(t)
	rec := e.do(t, http.MethodGet, "/api/v1/server", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without auth", rec.Code)
	}
	out := decode[map[string]any](t, rec)
	if out["name"] != "noted" || out["api_version"] != "v1" {
		t.Errorf("info = %v, want name noted api_version v1", out)
	}
	if v, _ := out["version"].(string); v == "" {
		t.Error("version is empty")
	}
	if _, ok := out["features"].(map[string]any); !ok {
		t.Errorf("features = %v, want an object", out["features"])
	}
}

func TestHealthz(t *testing.T) {
	e := newBareEnv(t)
	rec := e.do(t, http.MethodGet, "/healthz", "")
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	if rec.Header().Get(echo.HeaderXRequestID) == "" {
		t.Error("response has no request id header")
	}
}

func TestPanicIsRecoveredAs500(t *testing.T) {
	e := newBareEnv(t)
	e.srv.echo.GET("/boom", func(c *echo.Context) error { panic("boom") })
	rec := e.do(t, http.MethodGet, "/boom", "")
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Errorf("body %q leaks the panic value", rec.Body.String())
	}
}

func TestProtectedRoutesRequireAuth(t *testing.T) {
	e := newEnv(t)

	rec := e.doAnon(t, http.MethodGet, "/api/v1/vaults", "")
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "auth_required" {
		t.Errorf("no header = %d %s, want 401 auth_required", rec.Code, rec.Body.String())
	}

	saved := e.bearer
	e.bearer = "noted_not_a_real_token"
	rec = e.do(t, http.MethodGet, "/api/v1/vaults", "")
	e.bearer = saved
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "invalid_token" {
		t.Errorf("bad token = %d %s, want 401 invalid_token", rec.Code, rec.Body.String())
	}
}

func TestLoginFlow(t *testing.T) {
	e := newEnv(t)

	rec := e.doAnon(t, http.MethodPost, "/api/v1/login",
		`{"username":"admin","password":"`+testPassword+`","device_name":"curl"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("login = %d: %s", rec.Code, rec.Body.String())
	}
	out := decode[map[string]json.RawMessage](t, rec)
	var secret string
	if err := json.Unmarshal(out["token"], &secret); err != nil || !strings.HasPrefix(secret, "noted_") {
		t.Fatalf("token = %s, want a noted_ secret", out["token"])
	}

	e2 := &env{srv: e.srv, bearer: secret}
	if rec := e2.do(t, http.MethodGet, "/api/v1/vaults", ""); rec.Code != http.StatusOK {
		t.Errorf("minted token rejected: %d %s", rec.Code, rec.Body.String())
	}

	rec = e.doAnon(t, http.MethodPost, "/api/v1/login", `{"username":"admin","password":"wrong password"}`)
	if rec.Code != http.StatusUnauthorized || errCode(t, rec) != "invalid_credentials" {
		t.Errorf("bad login = %d %s, want 401 invalid_credentials", rec.Code, rec.Body.String())
	}
	rec = e.doAnon(t, http.MethodPost, "/api/v1/login", `{"username":"admin","password":"`+testPassword+`","kind":"mcp"}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("mcp kind via login = %d, want 400", rec.Code)
	}
}

func TestNoteWritesAreAttributed(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Mine","body":"x"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d: %s", rec.Code, rec.Body.String())
	}
	created := decode[noteJSON](t, rec)

	var byUser, byToken *string
	err := e.sqldb.QueryRow(`SELECT updated_by_user, updated_by_token FROM notes WHERE id = ?`, created.ID).
		Scan(&byUser, &byToken)
	if err != nil {
		t.Fatalf("query attribution: %v", err)
	}
	if byUser == nil || *byUser != e.adminID {
		t.Errorf("updated_by_user = %v, want admin %s", byUser, e.adminID)
	}
	if byToken == nil {
		t.Error("updated_by_token is NULL, want the device token id")
	}
}

func TestDevicesListAndRevoke(t *testing.T) {
	e := newEnv(t)

	rec := e.doAnon(t, http.MethodPost, "/api/v1/login",
		`{"username":"admin","password":"`+testPassword+`","device_name":"phone"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("second login = %d", rec.Code)
	}
	out := decode[map[string]json.RawMessage](t, rec)
	var phoneSecret string
	if err := json.Unmarshal(out["token"], &phoneSecret); err != nil {
		t.Fatalf("decode token: %v", err)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/devices", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list devices = %d: %s", rec.Code, rec.Body.String())
	}
	list := decode[map[string][]tokenJSON](t, rec)
	devices := list["devices"]
	if len(devices) != 2 {
		t.Fatalf("devices = %d entries, want 2", len(devices))
	}
	var phoneID string
	currentCount := 0
	for _, d := range devices {
		if d.Current {
			currentCount++
			if d.Name != "test device" {
				t.Errorf("current device = %q, want the caller's", d.Name)
			}
		}
		if d.Name == "phone" {
			phoneID = d.ID
		}
	}
	if currentCount != 1 || phoneID == "" {
		t.Fatalf("current flags = %d, phone found = %v", currentCount, phoneID != "")
	}

	if rec := e.do(t, http.MethodDelete, "/api/v1/devices/"+phoneID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d: %s", rec.Code, rec.Body.String())
	}
	phone := &env{srv: e.srv, bearer: phoneSecret}
	if rec := phone.do(t, http.MethodGet, "/api/v1/vaults", ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked device still works: %d", rec.Code)
	}
	rec = e.do(t, http.MethodGet, "/api/v1/devices", "")
	if got := decode[map[string][]tokenJSON](t, rec); len(got["devices"]) != 1 {
		t.Errorf("devices after revoke = %d, want 1", len(got["devices"]))
	}
	if rec := e.do(t, http.MethodDelete, "/api/v1/devices/"+phoneID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("double revoke = %d, want 404", rec.Code)
	}
}

func TestAdminUserManagement(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodPost, "/api/v1/users", `{"username":"bob","password":"`+testPassword+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user = %d: %s", rec.Code, rec.Body.String())
	}
	if created := decode[userJSON](t, rec); created.IsAdmin {
		t.Error("bob is an admin, want regular user")
	}

	rec = e.do(t, http.MethodGet, "/api/v1/users", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list users = %d", rec.Code)
	}
	if users := decode[map[string][]userJSON](t, rec)["users"]; len(users) != 2 {
		t.Errorf("users = %d, want 2", len(users))
	}

	rec = e.doAnon(t, http.MethodPost, "/api/v1/login", `{"username":"bob","password":"`+testPassword+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("bob login = %d", rec.Code)
	}
	out := decode[map[string]json.RawMessage](t, rec)
	var bobSecret string
	if err := json.Unmarshal(out["token"], &bobSecret); err != nil {
		t.Fatalf("decode bob token: %v", err)
	}
	bob := &env{srv: e.srv, bearer: bobSecret}

	if rec := bob.do(t, http.MethodGet, "/api/v1/users", ""); rec.Code != http.StatusForbidden || errCode(t, rec) != "admin_required" {
		t.Errorf("bob list users = %d %s, want 403 admin_required", rec.Code, rec.Body.String())
	}
	if rec := bob.do(t, http.MethodPost, "/api/v1/users", `{"username":"eve","password":"`+testPassword+`"}`); rec.Code != http.StatusForbidden {
		t.Errorf("bob create user = %d, want 403", rec.Code)
	}

	if rec := e.do(t, http.MethodPost, "/api/v1/users", `{"username":"bob","password":"`+testPassword+`"}`); rec.Code != http.StatusConflict || errCode(t, rec) != "username_taken" {
		t.Errorf("duplicate username = %d %s, want 409 username_taken", rec.Code, rec.Body.String())
	}
}

func TestVaultManagement(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodPost, "/api/v1/vaults", `{"name":"Work"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create vault = %d: %s", rec.Code, rec.Body.String())
	}
	created := decode[vaultJSON](t, rec)

	var owner *string
	if err := e.sqldb.QueryRow(`SELECT owner_id FROM vaults WHERE id = ?`, created.ID).Scan(&owner); err != nil {
		t.Fatalf("query owner: %v", err)
	}
	if owner == nil || *owner != e.adminID {
		t.Errorf("owner = %v, want %s", owner, e.adminID)
	}

	rec = e.do(t, http.MethodPatch, "/api/v1/vaults/"+created.ID, `{"name":"Work notes"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("rename = %d: %s", rec.Code, rec.Body.String())
	}
	if got := decode[vaultJSON](t, rec); got.Name != "Work notes" {
		t.Errorf("renamed to %q, want Work notes", got.Name)
	}

	if rec := e.do(t, http.MethodPost, "/api/v1/vaults", `{"name":"   "}`); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("blank vault name = %d, want 422", rec.Code)
	}

	if rec := e.do(t, http.MethodDelete, "/api/v1/vaults/"+created.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d: %s", rec.Code, rec.Body.String())
	}
	rec = e.do(t, http.MethodGet, "/api/v1/vaults", "")
	for _, v := range decode[vaultsListResponse](t, rec).Vaults {
		if v.ID == created.ID {
			t.Error("deleted vault still listed")
		}
	}
	if rec := e.do(t, http.MethodGet, "/api/v1/vaults/"+created.ID+"/notes", ""); rec.Code != http.StatusNotFound {
		t.Errorf("notes in deleted vault = %d, want 404", rec.Code)
	}
}

func TestListVaults(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodGet, "/api/v1/vaults", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	out := decode[vaultsListResponse](t, rec)
	if len(out.Vaults) != 1 || out.Vaults[0].ID != e.vaultID {
		t.Errorf("vaults = %+v, want the default vault %s", out.Vaults, e.vaultID)
	}
}

func TestCreateGetUpdateNote(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Hello","body":"# Hi\n"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}
	created := decode[noteJSON](t, rec)
	if created.Version != 1 || created.Name != "Hello" {
		t.Errorf("created = %+v, want version 1 name Hello", created)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/notes/"+created.ID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
	if got := decode[noteJSON](t, rec); got.Body != "# Hi\n" {
		t.Errorf("body = %q, want # Hi", got.Body)
	}

	rec = e.do(t, http.MethodPut, "/api/v1/notes/"+created.ID, `{"body":"# Hi v2\n","base_version":1}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := decode[noteJSON](t, rec); got.Version != 2 {
		t.Errorf("version = %d, want 2", got.Version)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes", "")
	out := decode[notesListResponse](t, rec)
	if len(out.Notes) != 1 || out.Notes[0].Version != 2 {
		t.Errorf("list = %+v, want one note at version 2", out.Notes)
	}
}

func TestAPIErrors(t *testing.T) {
	e := newEnv(t)
	if rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Dup"}`); rec.Code != http.StatusCreated {
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
		{"invalid name", http.MethodPost, "/api/v1/vaults/" + e.vaultID + "/notes", `{"name":"a/b"}`, 422, "invalid_name"},
		{"duplicate name", http.MethodPost, "/api/v1/vaults/" + e.vaultID + "/notes", `{"name":"Dup"}`, 409, "name_taken"},
		{"update missing body", http.MethodPut, "/api/v1/notes/none", `{"base_version":1}`, 400, "bad_request"},
		{"update missing base version", http.MethodPut, "/api/v1/notes/none", `{"body":"x"}`, 400, "bad_request"},
		{"malformed json", http.MethodPost, "/api/v1/vaults/" + e.vaultID + "/notes", `{`, 400, "bad_request"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := e.do(t, tt.method, tt.path, tt.body)
			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (%s)", rec.Code, tt.wantStatus, rec.Body.String())
			}
			if got := errCode(t, rec); got != tt.wantCode {
				t.Errorf("code = %q, want %q", got, tt.wantCode)
			}
		})
	}
}

func TestStaticApp(t *testing.T) {
	e := newBareEnv(t)

	rec := e.do(t, http.MethodGet, "/", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "noted test") {
		t.Errorf("GET / = %d %q, want the app shell", rec.Code, rec.Body.String())
	}

	rec = e.do(t, http.MethodGet, "/assets/app.js", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "console.log") {
		t.Errorf("GET asset = %d, want the file", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); !strings.Contains(cc, "immutable") {
		t.Errorf("asset Cache-Control = %q, want immutable", cc)
	}

	rec = e.do(t, http.MethodGet, "/notes/some-app-route", "")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "noted test") {
		t.Errorf("SPA fallback = %d %q, want index.html", rec.Code, rec.Body.String())
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("fallback Cache-Control = %q, want no-cache", cc)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/nope", "")
	if rec.Code != http.StatusNotFound {
		t.Errorf("unknown API route = %d, want JSON 404, got body %q", rec.Code, rec.Body.String())
	}
	if got := errCode(t, rec); got != "not_found" {
		t.Errorf("code = %q, want not_found", got)
	}
}

func TestFirstRunSetup(t *testing.T) {
	e := newBareEnv(t)

	rec := e.do(t, http.MethodGet, "/api/v1/setup", "")
	if out := decode[map[string]bool](t, rec); !out["needs_setup"] {
		t.Fatalf("needs_setup = false on a fresh server, want true")
	}

	rec = e.do(t, http.MethodPost, "/api/v1/setup", `{"username":"admin","password":"long enough password"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup status = %d: %s", rec.Code, rec.Body.String())
	}
	created := decode[userJSON](t, rec)
	if !created.IsAdmin || created.Username != "admin" {
		t.Errorf("created = %+v, want admin user", created)
	}
	if strings.Contains(rec.Body.String(), "argon2id") || strings.Contains(rec.Body.String(), "password") {
		t.Errorf("setup response leaks password material: %s", rec.Body.String())
	}

	rec = e.do(t, http.MethodGet, "/api/v1/setup", "")
	if out := decode[map[string]bool](t, rec); out["needs_setup"] {
		t.Error("needs_setup still true after setup")
	}

	rec = e.do(t, http.MethodPost, "/api/v1/setup", `{"username":"intruder","password":"long enough password"}`)
	if rec.Code != http.StatusConflict || errCode(t, rec) != "setup_complete" {
		t.Errorf("second setup = %d %s, want 409 setup_complete", rec.Code, rec.Body.String())
	}
}

func TestSetupValidation(t *testing.T) {
	e := newBareEnv(t)
	tests := []struct {
		body     string
		wantCode string
	}{
		{`{"username":"x","password":"long enough password"}`, "invalid_username"},
		{`{"username":"valid","password":"short"}`, "weak_password"},
		{`{"username":"valid","email":"nope","password":"long enough password"}`, "invalid_email"},
	}
	for _, tt := range tests {
		rec := e.do(t, http.MethodPost, "/api/v1/setup", tt.body)
		if rec.Code != http.StatusUnprocessableEntity || errCode(t, rec) != tt.wantCode {
			t.Errorf("body %s: got %d %s, want 422 %s", tt.body, rec.Code, errCode(t, rec), tt.wantCode)
		}
	}
}

func TestRenderPreview(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodPost, "/api/v1/render", `{"markdown":"| a |\n| - |\n| b |\n\n<script>x</script>"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	out := decode[map[string]string](t, rec)
	if !strings.Contains(out["html"], "<table>") {
		t.Errorf("html = %q, want a table", out["html"])
	}
	if strings.Contains(out["html"], "<script") {
		t.Errorf("html = %q leaks raw HTML", out["html"])
	}

	rec = e.do(t, http.MethodPost, "/api/v1/render", `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing markdown: status = %d, want 400", rec.Code)
	}
}

func TestNoteHTML(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Doc","body":"---\ntitle: x\n---\n# Rendered\n"}`)
	created := decode[noteJSON](t, rec)

	rec = e.do(t, http.MethodGet, "/api/v1/notes/"+created.ID+"/html", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	out := decode[map[string]any](t, rec)
	html, _ := out["html"].(string)
	if !strings.Contains(html, `<h1 id="rendered">Rendered</h1>`) {
		t.Errorf("html = %q, want rendered heading", html)
	}
	if strings.Contains(html, "title: x") {
		t.Errorf("html = %q leaks frontmatter", html)
	}

	if rec := e.do(t, http.MethodGet, "/api/v1/notes/none/html", ""); rec.Code != http.StatusNotFound {
		t.Errorf("missing note: status = %d, want 404", rec.Code)
	}
}

func TestListSinceCursor(t *testing.T) {
	e := newEnv(t)

	if rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"One","body":"1"}`); rec.Code != http.StatusCreated {
		t.Fatalf("seed: %d", rec.Code)
	}
	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Two","body":"2"}`)
	two := decode[noteJSON](t, rec)

	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes", "")
	full := decode[notesListResponse](t, rec)
	if len(full.Notes) != 2 || full.Cursor == 0 {
		t.Fatalf("full = %d notes cursor %d", len(full.Notes), full.Cursor)
	}

	if rec := e.do(t, http.MethodPut, "/api/v1/notes/"+two.ID, `{"body":"2b","base_version":1}`); rec.Code != http.StatusOK {
		t.Fatalf("update: %d", rec.Code)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes?since="+strconv.FormatInt(full.Cursor, 10), "")
	inc := decode[notesListResponse](t, rec)
	if len(inc.Notes) != 1 || inc.Notes[0].ID != two.ID {
		t.Errorf("incremental = %+v, want only note Two", inc.Notes)
	}
	if inc.Cursor <= full.Cursor {
		t.Errorf("cursor did not advance: %d -> %d", full.Cursor, inc.Cursor)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes?since="+strconv.FormatInt(inc.Cursor, 10), "")
	if empty := decode[notesListResponse](t, rec); len(empty.Notes) != 0 {
		t.Errorf("list since latest = %d notes, want 0", len(empty.Notes))
	}

	if rec := e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes?since=banana", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad since = %d, want 400", rec.Code)
	}
}

func TestTrashRestoreAndTombstones(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Doomed","body":"x"}`)
	note := decode[noteJSON](t, rec)

	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes", "")
	cursor := decode[notesListResponse](t, rec).Cursor

	if rec := e.do(t, http.MethodDelete, "/api/v1/notes/"+note.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("trash = %d: %s", rec.Code, rec.Body.String())
	}
	if rec := e.do(t, http.MethodGet, "/api/v1/notes/"+note.ID, ""); rec.Code != http.StatusNotFound {
		t.Errorf("get trashed = %d, want 404", rec.Code)
	}
	if rec := e.do(t, http.MethodPut, "/api/v1/notes/"+note.ID, `{"body":"y","base_version":1}`); rec.Code != http.StatusNotFound {
		t.Errorf("update trashed = %d, want 404", rec.Code)
	}
	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes", "")
	if full := decode[notesListResponse](t, rec); len(full.Notes) != 0 {
		t.Errorf("full list after trash = %d notes, want 0", len(full.Notes))
	}

	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes?since="+strconv.FormatInt(cursor, 10), "")
	inc := decode[notesListResponse](t, rec)
	if len(inc.Notes) != 1 || inc.Notes[0].ID != note.ID || !inc.Notes[0].Deleted {
		t.Errorf("tombstone stream = %+v, want the trashed note flagged deleted", inc.Notes)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/notes/"+note.ID+"/restore", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("restore = %d: %s", rec.Code, rec.Body.String())
	}
	if got := decode[noteJSON](t, rec); got.Body != "x" {
		t.Errorf("restored body = %q, want x", got.Body)
	}
	rec = e.do(t, http.MethodGet, "/api/v1/vaults/"+e.vaultID+"/notes", "")
	if full := decode[notesListResponse](t, rec); len(full.Notes) != 1 {
		t.Errorf("full list after restore = %d notes, want 1", len(full.Notes))
	}
}

func TestVaultTombstoneInStream(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodGet, "/api/v1/vaults", "")
	cursor := decode[vaultsListResponse](t, rec).Cursor

	rec = e.do(t, http.MethodPost, "/api/v1/vaults", `{"name":"Doomed vault"}`)
	vault := decode[vaultJSON](t, rec)
	if rec := e.do(t, http.MethodDelete, "/api/v1/vaults/"+vault.ID, ""); rec.Code != http.StatusNoContent {
		t.Fatalf("delete vault = %d", rec.Code)
	}

	rec = e.do(t, http.MethodGet, "/api/v1/vaults?since="+strconv.FormatInt(cursor, 10), "")
	inc := decode[vaultsListResponse](t, rec)
	found := false
	for _, v := range inc.Vaults {
		if v.ID == vault.ID {
			found = true
			if !v.Deleted {
				t.Error("deleted vault in stream not flagged deleted")
			}
		}
	}
	if !found {
		t.Errorf("deleted vault missing from since stream: %+v", inc.Vaults)
	}
}

func TestWikilinksResolveInRenderedNotes(t *testing.T) {
	e := newEnv(t)

	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Target","body":"# Target"}`)
	target := decode[noteJSON](t, rec)
	rec = e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes",
		`{"name":"Source","body":"[[Target]] and [[Ghost]]"}`)
	source := decode[noteJSON](t, rec)

	rec = e.do(t, http.MethodGet, "/api/v1/notes/"+source.ID+"/html", "")
	html, _ := decode[map[string]any](t, rec)["html"].(string)
	if !strings.Contains(html, `href="/notes/`+target.ID+`"`) {
		t.Errorf("existing note did not resolve: %s", html)
	}
	if !strings.Contains(html, `data-target="Ghost"`) || !strings.Contains(html, "unresolved") {
		t.Errorf("missing note is not marked unresolved: %s", html)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/render",
		`{"markdown":"[[Target]]","vault_id":"`+e.vaultID+`"}`)
	if got := decode[map[string]string](t, rec)["html"]; !strings.Contains(got, `href="/notes/`+target.ID+`"`) {
		t.Errorf("preview with vault did not resolve: %s", got)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/render", `{"markdown":"[[Target]]"}`)
	if got := decode[map[string]string](t, rec)["html"]; !strings.Contains(got, "unresolved") {
		t.Errorf("preview without vault should not resolve: %s", got)
	}

	rec = e.do(t, http.MethodPost, "/api/v1/render", `{"markdown":"[[Target]]","vault_id":"nope"}`)
	if rec.Code != http.StatusNotFound {
		t.Errorf("render against unknown vault = %d, want 404", rec.Code)
	}
}

func TestStaleUpdateConflicts(t *testing.T) {
	e := newEnv(t)
	rec := e.do(t, http.MethodPost, "/api/v1/vaults/"+e.vaultID+"/notes", `{"name":"Contested","body":"v1"}`)
	created := decode[noteJSON](t, rec)
	if rec := e.do(t, http.MethodPut, "/api/v1/notes/"+created.ID, `{"body":"v2","base_version":1}`); rec.Code != http.StatusOK {
		t.Fatalf("first update: %d", rec.Code)
	}
	rec = e.do(t, http.MethodPut, "/api/v1/notes/"+created.ID, `{"body":"lost","base_version":1}`)
	if rec.Code != http.StatusConflict {
		t.Errorf("status = %d, want 409", rec.Code)
	}
	if got := errCode(t, rec); got != "version_conflict" {
		t.Errorf("code = %q, want version_conflict", got)
	}
}
