package model

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/core/run/journal"
)

func TestSolvePreparationSetJournalPreservesExistingOwnership(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the first journal owner", func(t *testing.T) {
		t.Parallel()

		prep := &SolvePreparation{}
		firstDir := filepath.Join(t.TempDir(), "first")
		if err := os.MkdirAll(firstDir, 0o755); err != nil {
			t.Fatalf("mkdir first journal dir: %v", err)
		}
		first, err := journal.Open(filepath.Join(firstDir, "events.jsonl"), nil, 0)
		if err != nil {
			t.Fatalf("open first journal: %v", err)
		}
		defer func() {
			_ = first.Close(context.Background())
		}()

		secondDir := filepath.Join(t.TempDir(), "second")
		if err := os.MkdirAll(secondDir, 0o755); err != nil {
			t.Fatalf("mkdir second journal dir: %v", err)
		}
		second, err := journal.Open(filepath.Join(secondDir, "events.jsonl"), nil, 0)
		if err != nil {
			t.Fatalf("open second journal: %v", err)
		}
		defer func() {
			_ = second.Close(context.Background())
		}()

		prep.SetJournal(first)
		prep.SetJournal(second)

		if got := prep.Journal(); got != first {
			t.Fatalf("expected first journal ownership to be preserved, got %p want %p", got, first)
		}
	})
}

func TestSolvePreparationCloseJournalPreservesHandleOnFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve the handle when close fails", func(t *testing.T) {
		t.Parallel()

		closeErr := errors.New("close failed")
		handle := &stubJournalHandle{err: closeErr}
		prep := &SolvePreparation{JournalHandle: handle}

		err := prep.CloseJournal(context.Background())
		if !errors.Is(err, closeErr) {
			t.Fatalf("expected close error %v, got %v", closeErr, err)
		}
		if prep.JournalHandle != handle {
			t.Fatal("expected failed close to preserve the journal handle for retry")
		}
	})
}

type stubJournalHandle struct {
	err error
}

func (s *stubJournalHandle) Journal() *journal.Journal {
	return nil
}

func (s *stubJournalHandle) Close(context.Context) error {
	return s.err
}
