package markdown

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
)

type Renderer struct {
	md goldmark.Markdown
}

func NewRenderer() *Renderer {
	return &Renderer{md: goldmark.New(
		goldmark.WithExtensions(extension.GFM, extension.Footnote, &wikilinkExtension{}),
		goldmark.WithParserOptions(parser.WithAutoHeadingID()),
	)}
}

// Render converts a full note body (frontmatter included) to HTML. Raw HTML never passes through.
func (r *Renderer) Render(src []byte, resolver Resolver) ([]byte, error) {
	_, body := SplitFrontmatter(src)
	pc := parser.NewContext()
	if resolver != nil {
		pc.Set(resolverKey, resolver)
	}
	var buf bytes.Buffer
	if err := r.md.Convert(body, &buf, parser.WithContext(pc)); err != nil {
		return nil, fmt.Errorf("render markdown: %w", err)
	}
	return buf.Bytes(), nil
}

func SplitFrontmatter(src []byte) (frontmatter, body []byte) {
	lineEnd := bytes.IndexByte(src, '\n')
	if lineEnd < 0 || trimCR(src[:lineEnd]) != "---" {
		return nil, src
	}
	pos := lineEnd + 1
	for pos < len(src) {
		next := bytes.IndexByte(src[pos:], '\n')
		var line []byte
		lineLen := 0
		if next < 0 {
			line = src[pos:]
			lineLen = len(line)
		} else {
			line = src[pos : pos+next]
			lineLen = next + 1
		}
		if trimCR(line) == "---" {
			return src[lineEnd+1 : pos], src[pos+lineLen:]
		}
		if next < 0 {
			break
		}
		pos += lineLen
	}
	return nil, src
}

func trimCR(b []byte) string {
	return strings.TrimSuffix(string(b), "\r")
}
