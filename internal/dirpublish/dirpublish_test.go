package dirpublish

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/compozy/compozy/internal/fileutil"
)

func TestJournalPayloadBound(t *testing.T) {
	t.Parallel()

	t.Run("Should round-trip a payload at the size limit", func(t *testing.T) {
		t.Parallel()

		root := openTestRoot(t, t.TempDir())
		payload := bytes.Repeat([]byte{0xff}, MaxPayloadSize)
		journal, err := NewJournal("target", payload)
		if err != nil {
			t.Fatalf("NewJournal() error = %v", err)
		}
		if err := WriteJournal(root, journal, false); err != nil {
			t.Fatalf("WriteJournal() error = %v", err)
		}

		got, exists, err := ReadJournal(root, "target")
		if err != nil || !exists {
			t.Fatalf("ReadJournal() = exists %v, error %v; want existing journal", exists, err)
		}
		if !bytes.Equal(got.Payload, payload) {
			t.Fatalf("ReadJournal() payload length = %d, want %d identical bytes", len(got.Payload), len(payload))
		}
	})

	t.Run("Should reject a payload over the size limit", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		root := openTestRoot(t, parent)
		oversized := make([]byte, MaxPayloadSize+1)
		if _, err := NewJournal("target", oversized); !errors.Is(err, ErrPayloadTooLarge) {
			t.Fatalf("NewJournal(oversized) error = %v, want %v", err, ErrPayloadTooLarge)
		}
		journal, err := NewJournal("target", nil)
		if err != nil {
			t.Fatalf("NewJournal() error = %v", err)
		}
		journal.Payload = oversized
		if err := WriteJournal(root, journal, false); !errors.Is(err, ErrPayloadTooLarge) {
			t.Fatalf("WriteJournal(oversized) error = %v, want %v", err, ErrPayloadTooLarge)
		}
		if _, err := os.Stat(filepath.Join(parent, JournalName("target"))); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("Stat(journal) error = %v, want os.ErrNotExist", err)
		}
	})

	t.Run("Should require recovery for an on-disk payload over the size limit", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		root := openTestRoot(t, parent)
		journal, err := NewJournal("target", nil)
		if err != nil {
			t.Fatalf("NewJournal() error = %v", err)
		}
		journal.Payload = make([]byte, MaxPayloadSize+1)
		contents, err := json.Marshal(journal)
		if err != nil {
			t.Fatalf("json.Marshal(journal) error = %v", err)
		}
		if err := os.WriteFile(filepath.Join(parent, JournalName("target")), contents, 0o600); err != nil {
			t.Fatalf("WriteFile(journal) error = %v", err)
		}

		if _, _, err := ReadJournal(root, "target"); !errors.Is(err, ErrRecoveryRequired) {
			t.Fatalf("ReadJournal(oversized payload) error = %v, want %v", err, ErrRecoveryRequired)
		}
	})

	t.Run("Should omit the payload field when the payload is empty", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		root := openTestRoot(t, parent)
		journal, err := NewJournal("target", nil)
		if err != nil {
			t.Fatalf("NewJournal() error = %v", err)
		}
		if err := WriteJournal(root, journal, false); err != nil {
			t.Fatalf("WriteJournal() error = %v", err)
		}
		contents, err := os.ReadFile(filepath.Join(parent, JournalName("target")))
		if err != nil {
			t.Fatalf("ReadFile(journal) error = %v", err)
		}
		if bytes.Contains(contents, []byte(`"payload"`)) {
			t.Fatalf("journal = %s, want no payload field", contents)
		}
	})
}

func TestRecoverDispatchesLayoutToPolicy(t *testing.T) {
	t.Parallel()

	layouts := []Layout{
		0,
		HasTarget,
		HasIncoming,
		HasBackup,
		HasTarget | HasIncoming,
		HasTarget | HasBackup,
		HasIncoming | HasBackup,
		HasTarget | HasIncoming | HasBackup,
	}
	for _, layout := range layouts {
		t.Run(fmt.Sprintf("Should pass layout %03b to the policy", layout), func(t *testing.T) {
			t.Parallel()

			parent := t.TempDir()
			root := openTestRoot(t, parent)
			journal, err := NewJournal("target", []byte("caller state"))
			if err != nil {
				t.Fatalf("NewJournal() error = %v", err)
			}
			journal.Phase = PhaseBackup
			if err := WriteJournal(root, journal, false); err != nil {
				t.Fatalf("WriteJournal() error = %v", err)
			}
			for flag, name := range map[Layout]string{
				HasTarget:   journal.TargetName,
				HasIncoming: journal.IncomingName,
				HasBackup:   journal.BackupName,
			} {
				if layout&flag != 0 {
					if err := os.Mkdir(filepath.Join(parent, name), 0o700); err != nil {
						t.Fatalf("Mkdir(%q) error = %v", name, err)
					}
				}
			}
			policyErr := errors.New("policy decision")
			policy := &recordingPolicy{err: policyErr}

			if err := Recover(root, "target", policy); !errors.Is(err, policyErr) {
				t.Fatalf("Recover() error = %v, want policy error %v", err, policyErr)
			}
			if policy.calls != 1 {
				t.Fatalf("policy calls = %d, want 1", policy.calls)
			}
			if policy.layout != layout {
				t.Fatalf("policy layout = %03b, want %03b", policy.layout, layout)
			}
			if policy.journal.IncomingName != journal.IncomingName ||
				policy.journal.Phase != PhaseBackup ||
				string(policy.journal.Payload) != "caller state" {
				t.Fatalf("policy journal = %#v, want the on-disk journal %#v", policy.journal, journal)
			}
		})
	}

	t.Run("Should not call the policy without a journal", func(t *testing.T) {
		t.Parallel()

		policy := &recordingPolicy{}
		if err := Recover(openTestRoot(t, t.TempDir()), "target", policy); err != nil {
			t.Fatalf("Recover(no journal) error = %v", err)
		}
		if policy.calls != 0 {
			t.Fatalf("policy calls = %d, want 0", policy.calls)
		}
	})
}

func TestAdvancePhaseReportsWriteFailure(t *testing.T) {
	t.Parallel()

	t.Run("Should return a failed published write to the caller", func(t *testing.T) {
		t.Parallel()

		parent := t.TempDir()
		root := openTestRoot(t, parent)
		journal, err := NewJournal("target", nil)
		if err != nil {
			t.Fatalf("NewJournal() error = %v", err)
		}
		journal.Phase = PhaseBackup
		if err := WriteJournal(root, journal, false); err != nil {
			t.Fatalf("WriteJournal() error = %v", err)
		}
		journalPath := filepath.Join(parent, JournalName("target"))
		if err := os.Remove(journalPath); err != nil {
			t.Fatalf("Remove(journal) error = %v", err)
		}
		if err := os.MkdirAll(filepath.Join(journalPath, "occupied"), 0o700); err != nil {
			t.Fatalf("MkdirAll(journal blocker) error = %v", err)
		}

		if err := AdvancePhase(root, &journal, PhasePublished); err == nil {
			t.Fatal("AdvancePhase(published) error = nil, want write failure")
		}
		if journal.Phase != PhaseBackup {
			t.Fatalf("journal phase after failed write = %q, want %q", journal.Phase, PhaseBackup)
		}
	})

	t.Run("Should persist an advanced phase", func(t *testing.T) {
		t.Parallel()

		root := openTestRoot(t, t.TempDir())
		journal, err := NewJournal("target", nil)
		if err != nil {
			t.Fatalf("NewJournal() error = %v", err)
		}
		if err := WriteJournal(root, journal, false); err != nil {
			t.Fatalf("WriteJournal() error = %v", err)
		}
		if err := AdvancePhase(root, &journal, PhasePublished); err != nil {
			t.Fatalf("AdvancePhase(published) error = %v", err)
		}
		got, _, err := ReadJournal(root, "target")
		if err != nil {
			t.Fatalf("ReadJournal() error = %v", err)
		}
		if got.Phase != PhasePublished {
			t.Fatalf("persisted phase = %q, want %q", got.Phase, PhasePublished)
		}
	})
}

type recordingPolicy struct {
	calls   int
	journal Journal
	layout  Layout
	err     error
}

func (p *recordingPolicy) RecoverLayout(_ *fileutil.Directory, journal Journal, layout Layout) error {
	p.calls++
	p.journal = journal
	p.layout = layout
	return p.err
}

func openTestRoot(t *testing.T, path string) *fileutil.Directory {
	t.Helper()

	root, err := fileutil.OpenDirectoryForMutation(path)
	if err != nil {
		t.Fatalf("OpenDirectoryForMutation(%q) error = %v", path, err)
	}
	t.Cleanup(func() {
		if closeErr := root.Close(); closeErr != nil {
			t.Errorf("Directory.Close(%q) error = %v", path, closeErr)
		}
	})
	return root
}
