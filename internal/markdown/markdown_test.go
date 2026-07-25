package markdown

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestGolden(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no testdata files: %v", err)
	}
	r := NewRenderer()
	for _, f := range files {
		name := strings.TrimSuffix(filepath.Base(f), ".md")
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(f)
			if err != nil {
				t.Fatalf("read %s: %v", f, err)
			}
			got, err := r.Render(src)
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			golden := strings.TrimSuffix(f, ".md") + ".html"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatalf("write golden: %v", err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden %s (run with -update to create): %v", golden, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("output differs from %s\ngot:\n%s\nwant:\n%s", golden, got, want)
			}
		})
	}
}

func TestRawHTMLNeverPassesThrough(t *testing.T) {
	r := NewRenderer()
	for _, src := range []string{
		"<script>alert(1)</script>",
		"text with <img src=x onerror=alert(1)> inline",
		"[click](javascript:alert(1))",
		"<iframe src=\"https://evil.example\"></iframe>",
	} {
		out, err := r.Render([]byte(src))
		if err != nil {
			t.Fatalf("Render(%q): %v", src, err)
		}
		html := string(out)
		for _, bad := range []string{"<script", "onerror", "javascript:", "<iframe"} {
			if strings.Contains(html, bad) {
				t.Errorf("Render(%q) output contains %q: %s", src, bad, html)
			}
		}
	}
}

func TestSplitFrontmatter(t *testing.T) {
	tests := []struct {
		name, src, wantFM, wantBody string
	}{
		{"no frontmatter", "# Hi\n", "", "# Hi\n"},
		{"basic", "---\ntitle: x\n---\n# Hi\n", "title: x\n", "# Hi\n"},
		{"crlf", "---\r\ntitle: x\r\n---\r\nbody\r\n", "title: x\r\n", "body\r\n"},
		{"empty block", "---\n---\nbody", "", "body"},
		{"unterminated", "---\ntitle: x\nbody", "", "---\ntitle: x\nbody"},
		{"delimiter mid document", "# Hi\n---\n", "", "# Hi\n---\n"},
		{"hr after frontmatter", "---\na: 1\n---\ntext\n---\nmore", "a: 1\n", "text\n---\nmore"},
		{"closing at eof", "---\na: 1\n---", "a: 1\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm, body := SplitFrontmatter([]byte(tt.src))
			if string(fm) != tt.wantFM {
				t.Errorf("frontmatter = %q, want %q", fm, tt.wantFM)
			}
			if string(body) != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}
