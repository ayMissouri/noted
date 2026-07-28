package markdown

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLinksIn(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Link
	}{
		{
			name: "wikilinks in document order",
			src:  "[[First]] then [[Second#Part|shown]]",
			want: []Link{
				{Kind: LinkWikilink, Target: "First"},
				{Kind: LinkWikilink, Target: "Second", Heading: "Part", Display: "shown"},
			},
		},
		{
			name: "embeds",
			src:  "![[diagram.png]] and ![[Other note]]",
			want: []Link{
				{Kind: LinkEmbed, Target: "diagram.png"},
				{Kind: LinkEmbed, Target: "Other note"},
			},
		},
		{
			name: "relative markdown links count",
			src:  "[see](Other%20note.md) and [part](Other%20note.md#Heading)",
			want: []Link{
				{Kind: LinkMarkdown, Target: "Other note", Display: "see"},
				{Kind: LinkMarkdown, Target: "Other note", Heading: "Heading", Display: "part"},
			},
		},
		{
			name: "relative images are embeds",
			src:  "![alt](pictures/cat.png)",
			want: []Link{{Kind: LinkEmbed, Target: "pictures/cat.png", Display: "alt"}},
		},
		{
			name: "absolute urls are skipped",
			src:  "[site](https://example.com) <https://auto.example> ![x](http://example.com/a.png) [proto](//example.com)",
			want: nil,
		},
		{
			name: "bare fragments are skipped",
			src:  "[jump](#section)",
			want: nil,
		},
		{
			name: "frontmatter is not scanned",
			src:  "---\naliases: [[not a link]]\n---\n[[Real]]",
			want: []Link{{Kind: LinkWikilink, Target: "Real"}},
		},
		{
			name: "code spans are not links",
			src:  "`[[not a link]]` and [[Real]]",
			want: []Link{{Kind: LinkWikilink, Target: "Real"}},
		},
		{
			name: "no links",
			src:  "# Just a heading\n\nSome text.",
			want: nil,
		},
		{
			name: "mixed kinds keep document order",
			src:  "[[A]] then [b](B.md) then ![[C.png]]",
			want: []Link{
				{Kind: LinkWikilink, Target: "A"},
				{Kind: LinkMarkdown, Target: "B", Display: "b"},
				{Kind: LinkEmbed, Target: "C.png"},
			},
		},
	}

	r := NewRenderer()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.LinksIn([]byte(tt.src))
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("LinksIn mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
