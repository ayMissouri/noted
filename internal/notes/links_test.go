package notes

import (
	"context"
	"strings"
	"testing"

	"github.com/ayMissouri/noted/internal/storage/db"
)

func noteLinks(t *testing.T, s *Service, noteID string) []db.Link {
	t.Helper()
	links, err := s.q.ListNoteLinks(context.Background(), noteID)
	if err != nil {
		t.Fatalf("ListNoteLinks: %v", err)
	}
	return links
}

func TestSaveWritesLinks(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()

	note, err := s.Create(ctx, vaultID, "Source", "[[Target]] and ![[pic.png]] and [md](Other.md)", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	links := noteLinks(t, s, note.ID)
	if len(links) != 3 {
		t.Fatalf("links = %d, want 3: %+v", len(links), links)
	}
	want := []struct{ kind, target string }{
		{"wikilink", "Target"},
		{"embed", "pic.png"},
		{"markdown", "Other"},
	}
	for i, w := range want {
		if links[i].Kind != w.kind || links[i].TargetRaw != w.target {
			t.Errorf("link %d = %s/%s, want %s/%s", i, links[i].Kind, links[i].TargetRaw, w.kind, w.target)
		}
		if links[i].Ord != int64(i) {
			t.Errorf("link %d has ord %d", i, links[i].Ord)
		}
	}
}

func TestUpdateReplacesLinks(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()

	note, err := s.Create(ctx, vaultID, "Source", "[[One]] [[Two]]", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(noteLinks(t, s, note.ID)) != 2 {
		t.Fatalf("expected 2 links after create")
	}

	if _, err := s.Update(ctx, note.ID, note.Version, "[[Three]]", System); err != nil {
		t.Fatalf("Update: %v", err)
	}
	links := noteLinks(t, s, note.ID)
	if len(links) != 1 || links[0].TargetRaw != "Three" {
		t.Errorf("links after update = %+v, want only Three", links)
	}

	if _, err := s.Update(ctx, note.ID, note.Version+1, "no links at all", System); err != nil {
		t.Fatalf("second Update: %v", err)
	}
	if got := noteLinks(t, s, note.ID); len(got) != 0 {
		t.Errorf("links after removing them = %+v, want none", got)
	}
}

func TestResolverResolvesByName(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()

	target, err := s.Create(ctx, vaultID, "Target note", "", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	r := s.Resolver(ctx, vaultID)

	href, ok := r.ResolveWikilink("Target note", "", false)
	if !ok || href != "/notes/"+target.ID {
		t.Errorf("resolve = %q, %v; want /notes/%s", href, ok, target.ID)
	}
	if href, ok := r.ResolveWikilink("target NOTE", "", false); !ok || href != "/notes/"+target.ID {
		t.Errorf("case-insensitive resolve = %q, %v", href, ok)
	}
	if href, ok := r.ResolveWikilink("Target note", "Some heading", false); !ok || !strings.HasSuffix(href, "#Some%20heading") {
		t.Errorf("heading resolve = %q, %v", href, ok)
	}
	if _, ok := r.ResolveWikilink("Nothing here", "", false); ok {
		t.Error("unknown target resolved")
	}
	if href, ok := r.ResolveWikilink("", "Local heading", false); !ok || href != "#Local%20heading" {
		t.Errorf("same-note heading = %q, %v", href, ok)
	}
}

func TestResolverIgnoresTrashedAndOtherVaults(t *testing.T) {
	s, vaultID := testService(t)
	ctx := context.Background()
	otherVault := seedUserAndVault(t, s, "other-owner")

	note, err := s.Create(ctx, vaultID, "Findable", "", System)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := s.Create(ctx, otherVault, "Elsewhere", "", actorFor("other-owner")); err != nil {
		t.Fatalf("Create in other vault: %v", err)
	}

	r := s.Resolver(ctx, vaultID)
	if _, ok := r.ResolveWikilink("Elsewhere", "", false); ok {
		t.Error("resolved a note from another vault")
	}
	if err := s.Trash(ctx, note.ID, System); err != nil {
		t.Fatalf("Trash: %v", err)
	}
	if _, ok := r.ResolveWikilink("Findable", "", false); ok {
		t.Error("resolved a trashed note")
	}
	if _, err := s.Restore(ctx, note.ID, System); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if _, ok := r.ResolveWikilink("Findable", "", false); !ok {
		t.Error("restored note does not resolve")
	}
}
