package markdown

import (
	"strings"
	"testing"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

// parseOne renders src and returns the first wikilink found.
func parseOne(t *testing.T, src string) *WikiLink {
	t.Helper()
	r := NewRenderer()
	doc := r.md.Parser().Parse(text.NewReader([]byte(src)))
	var found *WikiLink
	err := ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if w, ok := n.(*WikiLink); ok && entering && found == nil {
			found = w
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	return found
}

func TestWikilinkParsing(t *testing.T) {
	tests := []struct {
		name, src                       string
		target, heading, display, label string
		embed                           bool
	}{
		{name: "plain", src: "[[Note name]]", target: "Note name", label: "Note name"},
		{name: "heading", src: "[[Note#Section]]", target: "Note", heading: "Section", label: "Note > Section"},
		{name: "display", src: "[[Note|shown]]", target: "Note", display: "shown", label: "shown"},
		{name: "heading and display", src: "[[Note#Sec|shown]]", target: "Note", heading: "Sec", display: "shown", label: "shown"},
		{name: "same note heading", src: "[[#Section]]", heading: "Section", label: "Section"},
		{name: "embed", src: "![[image.png]]", target: "image.png", label: "image.png", embed: true},
		{name: "embed with display", src: "![[doc.pdf|The doc]]", target: "doc.pdf", display: "The doc", label: "The doc", embed: true},
		{name: "spaces trimmed", src: "[[  Note  #  Sec  |  shown  ]]", target: "Note", heading: "Sec", display: "shown", label: "shown"},
		{name: "second pipe is display text", src: "[[Note|a|b]]", target: "Note", display: "a|b", label: "a|b"},
		{name: "inside a sentence", src: "see [[Other note]] for more", target: "Other note", label: "Other note"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseOne(t, tt.src)
			if got == nil {
				t.Fatalf("no wikilink parsed from %q", tt.src)
			}
			if string(got.Target) != tt.target {
				t.Errorf("Target = %q, want %q", got.Target, tt.target)
			}
			if string(got.Heading) != tt.heading {
				t.Errorf("Heading = %q, want %q", got.Heading, tt.heading)
			}
			if string(got.Display) != tt.display {
				t.Errorf("Display = %q, want %q", got.Display, tt.display)
			}
			if string(got.Label()) != tt.label {
				t.Errorf("Label = %q, want %q", got.Label(), tt.label)
			}
			if got.Embed != tt.embed {
				t.Errorf("Embed = %v, want %v", got.Embed, tt.embed)
			}
		})
	}
}

func TestNotWikilinks(t *testing.T) {
	for _, src := range []string{
		"[[]]",
		"[[|]]",
		"[[#]]",
		"[[unclosed",
		"[not a wikilink]",
		"[link](https://example.com)",
		"![alt](image.png)",
		"[[\n]]",
	} {
		if got := parseOne(t, src); got != nil {
			t.Errorf("%q parsed as a wikilink: target %q", src, got.Target)
		}
	}
}

func TestNestedBracketsMatchTheInnerLink(t *testing.T) {
	got := parseOne(t, "[[nested [[inner]] ]]")
	if got == nil || string(got.Target) != "inner" {
		t.Errorf("got %v, want the inner link", got)
	}
}

func TestStandardMarkdownLinksStillWork(t *testing.T) {
	r := NewRenderer()
	out, err := r.Render([]byte("[text](https://example.com) and ![alt](pic.png)"), nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, `<a href="https://example.com">text</a>`) {
		t.Errorf("markdown link broken: %s", html)
	}
	if !strings.Contains(html, `<img src="pic.png" alt="alt"`) {
		t.Errorf("markdown image broken: %s", html)
	}
}

type fakeResolver struct{ known map[string]string }

func (f fakeResolver) ResolveWikilink(target, heading string, embed bool) (string, bool) {
	href, ok := f.known[target]
	if !ok {
		return "", false
	}
	if heading != "" {
		href += "#" + heading
	}
	return href, true
}

func TestWikilinkRendering(t *testing.T) {
	r := NewRenderer()
	resolver := fakeResolver{known: map[string]string{"Real note": "/notes/abc"}}

	out, err := r.Render([]byte("[[Real note]] and [[Ghost note]]"), resolver)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := string(out)
	if !strings.Contains(html, `<a class="wikilink" href="/notes/abc">Real note</a>`) {
		t.Errorf("resolved link wrong: %s", html)
	}
	if !strings.Contains(html, `<a class="wikilink unresolved" data-target="Ghost note">Ghost note</a>`) {
		t.Errorf("unresolved link wrong: %s", html)
	}

	out, err = r.Render([]byte("[[Real note#Part two|see this]]"), resolver)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(out), `href="/notes/abc#Part two">see this</a>`) {
		t.Errorf("heading link wrong: %s", out)
	}

	out, err = r.Render([]byte("[[Real note]]"), nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(out), "unresolved") {
		t.Errorf("nil resolver should leave links unresolved: %s", out)
	}
}

func TestWikilinkEscaping(t *testing.T) {
	r := NewRenderer()
	out, err := r.Render([]byte(`[[evil" onmouseover=alert(1)]] and [[x|<script>bad</script>]]`), nil)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	html := string(out)
	for _, bad := range []string{`" onmouseover`, "<script>"} {
		if strings.Contains(html, bad) {
			t.Errorf("output contains unescaped %q: %s", bad, html)
		}
	}
}
