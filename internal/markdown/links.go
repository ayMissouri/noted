package markdown

import (
	"net/url"
	"strings"

	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
)

const (
	LinkWikilink = "wikilink"
	LinkEmbed    = "embed"
	LinkMarkdown = "markdown"
)

// Link is one outgoing reference found in a note body.
type Link struct {
	Kind    string
	Target  string
	Heading string
	Display string
}

func (r *Renderer) LinksIn(src []byte) []Link {
	_, body := SplitFrontmatter(src)
	doc := r.md.Parser().Parse(text.NewReader(body))

	var links []Link
	_ = ast.Walk(doc, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch node := n.(type) {
		case *WikiLink:
			kind := LinkWikilink
			if node.Embed {
				kind = LinkEmbed
			}
			links = append(links, Link{
				Kind:    kind,
				Target:  string(node.Target),
				Heading: string(node.Heading),
				Display: string(node.Display),
			})
		case *ast.Link:
			if target, heading, ok := internalTarget(node.Destination); ok {
				links = append(links, Link{
					Kind:    LinkMarkdown,
					Target:  target,
					Heading: heading,
					Display: string(node.Text(body)),
				})
			}
		case *ast.Image:
			if target, heading, ok := internalTarget(node.Destination); ok {
				links = append(links, Link{
					Kind:    LinkEmbed,
					Target:  target,
					Heading: heading,
					Display: string(node.Text(body)),
				})
			}
		}
		return ast.WalkContinue, nil
	})
	return links
}

func internalTarget(dest []byte) (target, heading string, ok bool) {
	raw := string(dest)
	if raw == "" || strings.HasPrefix(raw, "//") || strings.HasPrefix(raw, "#") {
		return "", "", false
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "" || u.Host != "" {
		return "", "", false
	}
	path := u.Path
	if decoded, err := url.PathUnescape(path); err == nil {
		path = decoded
	}
	path = strings.TrimSuffix(path, ".md")
	if path == "" {
		return "", "", false
	}
	return path, u.Fragment, true
}
