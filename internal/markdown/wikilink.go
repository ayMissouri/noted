package markdown

import (
	"bytes"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

type Resolver interface {
	ResolveWikilink(target, heading string, embed bool) (href string, ok bool)
}

var resolverKey = parser.NewContextKey()

var KindWikiLink = ast.NewNodeKind("WikiLink")

// WikiLink is [[target#heading|display]], or ![[target]] when Embed is set.
type WikiLink struct {
	ast.BaseInline
	Target   []byte
	Heading  []byte
	Display  []byte
	Embed    bool
	Href     string
	Resolved bool
}

func (n *WikiLink) Kind() ast.NodeKind { return KindWikiLink }

func (n *WikiLink) Dump(source []byte, level int) {
	ast.DumpHelper(n, source, level, map[string]string{
		"Target":  string(n.Target),
		"Heading": string(n.Heading),
		"Display": string(n.Display),
	}, nil)
}

// Label is the text shown for the link.
func (n *WikiLink) Label() []byte {
	switch {
	case len(n.Display) > 0:
		return n.Display
	case len(n.Target) > 0 && len(n.Heading) > 0:
		return append(append(append([]byte{}, n.Target...), " > "...), n.Heading...)
	case len(n.Heading) > 0:
		return n.Heading
	default:
		return n.Target
	}
}

type wikilinkParser struct{}

func (p *wikilinkParser) Trigger() []byte { return []byte{'[', '!'} }

func (p *wikilinkParser) Parse(parent ast.Node, block text.Reader, pc parser.Context) ast.Node {
	line, _ := block.PeekLine()
	if len(line) == 0 {
		return nil
	}
	open := 0
	embed := false
	if line[0] == '!' {
		embed = true
		open = 1
	}
	if len(line) < open+4 || line[open] != '[' || line[open+1] != '[' {
		return nil
	}
	inner := line[open+2:]
	end := bytes.Index(inner, []byte("]]"))
	if end < 0 {
		return nil
	}
	content := inner[:end]
	if len(content) == 0 || bytes.ContainsAny(content, "[]\n") {
		return nil
	}

	target, heading, display := splitWikilink(content)
	if len(target) == 0 && len(heading) == 0 {
		return nil
	}

	node := &WikiLink{Target: target, Heading: heading, Display: display, Embed: embed}
	if r, ok := pc.Get(resolverKey).(Resolver); ok && r != nil {
		node.Href, node.Resolved = r.ResolveWikilink(string(target), string(heading), embed)
	}
	block.Advance(open + 2 + end + 2)
	return node
}

func splitWikilink(content []byte) (target, heading, display []byte) {
	target = content
	if i := bytes.IndexByte(content, '|'); i >= 0 {
		target, display = content[:i], content[i+1:]
	}
	if i := bytes.IndexByte(target, '#'); i >= 0 {
		heading = target[i+1:]
		target = target[:i]
	}
	return bytes.TrimSpace(target), bytes.TrimSpace(heading), bytes.TrimSpace(display)
}

type wikilinkRenderer struct{}

func (r *wikilinkRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(KindWikiLink, r.render)
}

func (r *wikilinkRenderer) render(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*WikiLink)
	class := "wikilink"
	if n.Embed {
		class += " embed"
	}
	_, _ = w.WriteString(`<a class="` + class)
	if n.Resolved {
		_, _ = w.WriteString(`" href="`)
		_, _ = w.Write(util.EscapeHTML([]byte(n.Href)))
	} else {
		_, _ = w.WriteString(` unresolved" data-target="`)
		_, _ = w.Write(util.EscapeHTML(n.Target))
	}
	_, _ = w.WriteString(`">`)
	_, _ = w.Write(util.EscapeHTML(n.Label()))
	_, _ = w.WriteString(`</a>`)
	return ast.WalkSkipChildren, nil
}

type wikilinkExtension struct{}

func (e *wikilinkExtension) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&wikilinkParser{}, 199),
	))
	m.Renderer().AddOptions(renderer.WithNodeRenderers(
		util.Prioritized(&wikilinkRenderer{}, 199),
	))
}
