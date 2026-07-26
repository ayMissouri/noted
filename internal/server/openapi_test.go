package server

import (
	"net/http"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPICoversAllRoutes(t *testing.T) {
	e := newBareEnv(t)

	var doc struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(openapiSpec, &doc); err != nil {
		t.Fatalf("parse openapi.yaml: %v", err)
	}
	specSet := map[string]bool{}
	for path, ops := range doc.Paths {
		for method := range ops {
			switch method {
			case "parameters", "summary", "description":
				continue
			}
			specSet[strings.ToUpper(method)+" "+path] = true
		}
	}

	routeSet := map[string]bool{}
	for _, r := range e.srv.echo.Router().Routes() {
		if strings.HasSuffix(r.Path, "/*") {
			continue
		}
		path := r.Path
		for _, param := range r.Parameters {
			path = strings.Replace(path, ":"+param, "{"+param+"}", 1)
		}
		routeSet[r.Method+" "+path] = true
	}

	for route := range routeSet {
		if !specSet[route] {
			t.Errorf("route %s is not documented in openapi.yaml", route)
		}
	}
	for route := range specSet {
		if !routeSet[route] {
			t.Errorf("openapi.yaml documents %s but the server does not serve it", route)
		}
	}
}

func TestOpenAPIServed(t *testing.T) {
	e := newBareEnv(t)
	rec := e.do(t, http.MethodGet, "/api/v1/openapi.yaml", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 without auth", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "yaml") {
		t.Errorf("content type = %q, want yaml", ct)
	}
	if !strings.Contains(rec.Body.String(), "openapi:") {
		t.Error("body does not look like an OpenAPI document")
	}
}
