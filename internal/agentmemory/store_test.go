package agentmemory

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"errors"
)

func TestStore(t *testing.T) {
	key := Key{ProfileID: "profile-1", Origin: "user", Agent: "triage"}
	now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
	t.Run("Should persist scoped entries and redact every write path", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		s := NewStore(home)
		empty, err := s.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if empty.Content != "" {
			t.Fatalf("expected empty: %v", empty.Content)
		}
		if empty.Digest != "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
			t.Fatalf("empty document digest = %q", empty.Digest)
		}
		doc, err := s.Add(key, now, "workspace-a", "Remember preview", "")
		if err != nil {
			t.Fatal(err)
		}
		if doc.Content != "- 2026-11-01 · workspace-a · Remember preview\n" {
			t.Fatalf("want %v, got %v", "- 2026-11-01 · workspace-a · Remember preview\n", doc.Content)
		}
		doc, err = s.Write(key, "token=sk-abcdefghijklmnopqrstuvwxyz1234567890", doc.Digest)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(doc.Content, "sk-abcdefghijklmnopqrstuvwxyz1234567890") {
			t.Fatalf("unexpected text in %q", doc.Content)
		}
		_, err = s.Write(key, "stale", empty.Digest)
		if !errors.Is(err, ErrConflict) {
			t.Fatalf("unexpected error: %v", err)
		}
		history, err := s.History(key)
		if err != nil {
			t.Fatal(err)
		}
		if len(history) != 3 {
			t.Fatalf("unexpected length: %d", len(history))
		}
		if history[0].Digest != empty.Digest || strings.Contains(history[0].ID, ":") {
			t.Fatalf("history digest or revision ID = %+v", history[0])
		}
		restored, err := s.Rollback(key, history[1].ID)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(restored.Content, "Remember preview") {
			t.Fatalf("missing expected text in %q", restored.Content)
		}
		reopened, err := NewStore(home).Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if restored != reopened {
			t.Fatalf("want %v, got %v", restored, reopened)
		}
	})
	t.Run("Should archive oldest dates while preserving undated notes", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		s := NewStore(home)
		initial := "Operator preference\n- 2020-01-01 · Global · oldest\n" + strings.Repeat(
			"- 2026-10-31 · Global · recent\n",
			198,
		)
		doc, err := s.Write(key, initial, digest(""))
		if err != nil {
			t.Fatal(err)
		}
		if doc.Budget.Lines != 200 {
			t.Fatalf("want %v, got %v", 200, doc.Budget.Lines)
		}
		doc, err = s.Add(key, now, "Global", "new", "")
		if err != nil {
			t.Fatal(err)
		}
		if doc.Budget.Lines != 200 {
			t.Fatalf("want %v, got %v", 200, doc.Budget.Lines)
		}
		if !strings.Contains(doc.Content, "Operator preference") {
			t.Fatalf("missing expected text in %q", doc.Content)
		}
		if strings.Contains(doc.Content, "oldest") {
			t.Fatalf("unexpected text in %q", doc.Content)
		}
		results, err := s.Search(key, "oldest")
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 1 {
			t.Fatalf("unexpected length: %d", len(results))
		}
		if results[0].File != "archive.md" {
			t.Fatalf("want %v, got %v", "archive.md", results[0].File)
		}
	})
	t.Run("Should bound oversized undated snapshots without modifying notes", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		s := NewStore(home)
		_, err := s.Add(key, now, "Global", "seed", "")
		if err != nil {
			t.Fatal(err)
		}
		dir, err := s.directory(key)
		if err != nil {
			t.Fatal(err)
		}
		original := strings.Repeat("Operator note\n", 201)
		if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte(original), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := s.Load(key)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(snapshot, TruncationMarker) {
			t.Fatalf("missing expected text in %q", snapshot)
		}
		if !(fits(snapshot)) {
			t.Fatal("condition failed")
		}
		_, err = s.Add(key, now, "Global", "new", "")
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatalf("unexpected error: %v", err)
		}
		unchanged, err := s.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if original != unchanged.Content {
			t.Fatalf("want %v, got %v", original, unchanged.Content)
		}
		_, err = s.Write(key, "repaired\n", unchanged.Digest)
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should preserve parallel adds and isolate definition origins", func(t *testing.T) {
		t.Parallel()
		s := NewStore(t.TempDir())
		var wg sync.WaitGroup
		for i := range 20 {
			wg.Go(func() {
				_, err := s.Add(key, now, "Global", fmt.Sprintf("note %d", i), "")
				if err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		doc, err := s.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Budget.Lines != 20 {
			t.Fatalf("want %v, got %v", 20, doc.Budget.Lines)
		}
		other := key
		other.Origin = "workspace-a"
		doc, err = s.Read(other)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Content != "" {
			t.Fatalf("expected empty: %v", doc.Content)
		}
		if err := s.Retire(key, now); err != nil {
			t.Fatal(err)
		}
		doc, err = s.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if doc.Content != "" {
			t.Fatalf("expected empty: %v", doc.Content)
		}
		_, err = s.Add(key, now, "Global", "fresh", "")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should rank lexical matches and support replacement and removal", func(t *testing.T) {
		t.Parallel()
		s := NewStore(t.TempDir())
		_, err := s.Add(key, now, "Global", "preview", "")
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Add(key, now, "Global", "preview deployment", "")
		if err != nil {
			t.Fatal(err)
		}
		results, err := s.Search(key, "preview deployment")
		if err != nil {
			t.Fatal(err)
		}
		if len(results) != 2 {
			t.Fatalf("unexpected length: %d", len(results))
		}
		if !strings.Contains(results[0].Text, "preview deployment") {
			t.Fatalf("missing expected text in %q", results[0].Text)
		}
		doc, err := s.Replace(key, "deployment", "release")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(doc.Content, "preview release") {
			t.Fatalf("missing expected text in %q", doc.Content)
		}
		doc, count, err := s.Remove(key, "release")
		if err != nil {
			t.Fatal(err)
		}
		if count != 1 {
			t.Fatalf("want %v, got %v", 1, count)
		}
		if strings.Contains(doc.Content, "release") {
			t.Fatalf("unexpected text in %q", doc.Content)
		}
		_, err = s.Add(key, now, "Global", strings.Repeat("界", 1001), "")
		if !errors.Is(err, ErrEntryTooLarge) {
			t.Fatalf("unexpected error: %v", err)
		}
		_, err = s.Add(key, now, "Global", strings.Repeat("界", 1000), "2026-12-01")
		if err != nil {
			t.Fatal(err)
		}
	})
	t.Run("Should bound replacement results and preserve notes after rejection", func(t *testing.T) {
		t.Parallel()
		s := NewStore(t.TempDir())
		before, err := s.Add(key, now, "Global", "xx", "2026-12-01")
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.Replace(key, "x", strings.Repeat("界", 501))
		if !errors.Is(err, ErrEntryTooLarge) {
			t.Fatalf("replacement error = %v, want entry limit", err)
		}
		after, err := s.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if after.Content != before.Content || after.Digest != before.Digest {
			t.Fatal("rejected replacement changed memory")
		}
		after, err = s.Replace(key, "x", strings.Repeat("界", 500))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(after.Content, strings.Repeat("界", 1000)+" · until 2026-12-01") {
			t.Fatalf("boundary replacement lost text or expiry: %q", after.Content)
		}
	})
	t.Run("Should retain only fifty revisions and reject path escape", func(t *testing.T) {
		t.Parallel()
		s := NewStore(t.TempDir())
		for i := range 55 {
			_, err := s.Add(key, now, "Global", fmt.Sprintf("note %d", i), "")
			if err != nil {
				t.Fatal(err)
			}
		}
		history, err := s.History(key)
		if err != nil {
			t.Fatal(err)
		}
		if len(history) != 50 {
			t.Fatalf("unexpected length: %d", len(history))
		}
		_, err = s.Rollback(key, "../../secret")
		if !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected error: %v", err)
		}
		bad := key
		bad.Agent = "../escape"
		_, err = s.Read(bad)
		if err == nil {
			t.Fatal("expected error")
		}
	})
	t.Run("Should preserve main notes if archive cannot be written", func(t *testing.T) {
		t.Parallel()
		s := NewStore(t.TempDir())
		original := strings.Repeat("- 2026-10-31 · Global · note\n", 200)
		_, err := s.Write(key, original, digest(""))
		if err != nil {
			t.Fatal(err)
		}
		dir, err := s.directory(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "archive.md"), 0o700); err != nil {
			t.Fatal(err)
		}
		_, err = s.Add(key, now, "Global", "new", "")
		if err == nil {
			t.Fatal("expected error")
		}
		doc, err := s.Read(key)
		if err != nil {
			t.Fatal(err)
		}
		if original != doc.Content {
			t.Fatalf("want %v, got %v", original, doc.Content)
		}
	})
}

func TestMemoryBoundaries(t *testing.T) {
	key := Key{ProfileID: "profile-1", Origin: "user", Agent: "triage"}
	t.Run("Should archive oversized loaded files and tolerate interrupted archival", func(t *testing.T) {
		t.Parallel()
		s := NewStore(t.TempDir())
		dir, err := s.directory(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		oldest := "- 2020-01-01 · Global · oldest\n"
		initial := oldest + strings.Repeat("- 2026-10-31 · Global · recent\n", 200)
		if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
		// Recreate a crash after the archive write but before replacing the main file.
		if err := os.WriteFile(filepath.Join(dir, "archive.md"), []byte(oldest), 0o600); err != nil {
			t.Fatal(err)
		}
		snapshot, err := s.Load(key)
		if err != nil {
			t.Fatal(err)
		}
		if !fits(snapshot) || strings.Contains(snapshot, "oldest") {
			t.Fatalf("invalid loaded snapshot: %q", snapshot)
		}
		results, err := s.Search(key, "oldest")
		if err != nil {
			t.Fatal(err)
		}
		if len(results) == 0 {
			t.Fatal("archived note lost")
		}
	})
	t.Run("Should enforce byte budgets and validate entry dates", func(t *testing.T) {
		t.Parallel()
		_, _, err := Overflow(strings.Repeat("界", MaxBytes))
		if !errors.Is(err, ErrBudgetExceeded) {
			t.Fatal(err)
		}
		content := "- 2020-01-01 · Global · " + strings.Repeat("x", MaxBytes) + "\nPreference\n"
		kept, archived, err := Overflow(content)
		if err != nil {
			t.Fatal(err)
		}
		if kept != "Preference\n" || !strings.Contains(archived, "2020-01-01") {
			t.Fatal("wrong overflow selection")
		}
		for _, entry := range []struct{ scope, text, until string }{{"Global", "two\nlines", ""}, {"", "note", ""}, {"Global", "note", "bad-date"}} {
			if _, err := FormatEntry(
				time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
				entry.scope,
				entry.text,
				entry.until,
			); err == nil {
				t.Fatal("accepted invalid entry")
			}
		}
	})
	t.Run("Should reject symlinked memory and revision files", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		s := NewStore(home)
		dir, err := s.directory(key)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		outside := filepath.Join(t.TempDir(), "secret")
		if err := os.WriteFile(outside, []byte("private"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(outside, filepath.Join(dir, "MEMORY.md")); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Read(key); err == nil {
			t.Fatal("read symlink")
		}
		if err := os.Symlink(filepath.Dir(outside), filepath.Join(dir, "revisions")); err != nil {
			t.Fatal(err)
		}
		if _, err := s.History(key); err == nil {
			t.Fatal("read symlinked history")
		}
	})
}
