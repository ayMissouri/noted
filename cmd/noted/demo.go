package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/ayMissouri/noted/internal/notes"
)

var demoNotes = []struct {
	name string
	body string
}{
	{"Welcome to noted", `---
tags: [meta]
---
# Welcome to noted

Your notes live in vaults, written in plain Markdown.

- [x] Install noted
- [x] Open the web client
- [ ] Read the [[Markdown tour]]
- [ ] Empty the [[Reading list]]

Everything you write here can be exported as plain files at any time.
`},
	{"Markdown tour", `---
tags: [meta]
---
# Markdown tour

## Tables

| Syntax    | Renders as |
| --------- | ---------- |
| **bold**  | **bold**   |
| ~~strike~~ | ~~strike~~ |

## Code

` + "```go" + `
func hello() string {
	return "world"
}
` + "```" + `

## Footnotes

Markdown supports footnotes.[^1]

[^1]: Like this one.

Link between notes with wikilinks: [[Welcome to noted]].
`},
	{"Ideas", `# Ideas

- A garden journal with photos per plant
- Weekly review template, see [[Markdown tour]] for syntax
- Merge the [[Reading list]] into per-topic notes
`},
	{"Reading list", `---
tags: [reading]
---
# Reading list

| Title                        | Status  |
| ---------------------------- | ------- |
| Diary of a Wimpy Kid         | done    |
| The Hobbit                   | reading |
| The Fellowship of the Ring   | queued  |
`},
	{"Meeting notes 2026-07-20", `---
tags: [work]
---
# Meeting notes 2026-07-20

Attendees: me, future me.

Decisions:

- Keep notes in [[Ideas]] until they earn their own page.
- Review the [[Reading list]] monthly.
`},
}

func demo() error {
	a, cleanup, err := setup()
	if err != nil {
		return err
	}
	defer cleanup()

	ctx := context.Background()
	vault, err := a.notes.EnsureDefaultVault(ctx)
	if err != nil {
		return err
	}

	created, existing := 0, 0
	for _, n := range demoNotes {
		_, err := a.notes.Create(ctx, vault.ID, n.name, n.body, notes.System)
		switch {
		case errors.Is(err, notes.ErrNameTaken):
			existing++
		case err != nil:
			return fmt.Errorf("seed %q: %w", n.name, err)
		default:
			created++
		}
	}
	fmt.Printf("seeded vault %q: %d notes created, %d already there\n", vault.Name, created, existing)
	return nil
}
