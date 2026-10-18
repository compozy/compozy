package worktree

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	deliveryPhasePrepared   = "prepared"
	deliveryPhaseCommitting = "committing"
	deliveryPhaseCommitted  = "committed"
	deliveryPhasePushing    = "pushing"
)

type managedDeliveryJournal struct {
	StagedScope   string            `json:"staged_scope,omitempty"`
	Failure       string            `json:"failure,omitempty"`
	ReviewedScope ExitCommitScope   `json:"reviewed_scope"`
	Version       int               `json:"version"`
	OperationID   string            `json:"operation_id"`
	SessionID     string            `json:"session_id"`
	Item          Worktree          `json:"worktree"`
	Request       ExitActionRequest `json:"request"`
	OriginalHead  string            `json:"original_head"`
	Head          string            `json:"head,omitempty"`
	BaseHead      string            `json:"base_head"`
	Snapshot      string            `json:"snapshot"`
	Tree          string            `json:"tree,omitempty"`
	RemoteURLs    []string          `json:"remote_urls"`
	Phase         string            `json:"phase"`
	Result        ExitActionResult  `json:"result"`
}

func (s *Service) deliveryJournalPath(workspaceID, worktreeID, deliveryID string) (string, error) {
	if s.root == "" {
		return "", refusal(ErrConfigInvalid, "Delivery journal root is unavailable.")
	}
	sum := sha256.Sum256([]byte(workspaceID + "\x00" + worktreeID + "\x00" + deliveryID))
	return filepath.Join(s.root, ".delivery", hex.EncodeToString(sum[:])+".json"), nil
}

func readDeliveryJournal(path string) (*managedDeliveryJournal, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var journal managedDeliveryJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return nil, err
	}
	if journal.Version != 1 {
		return nil, refusal(ErrSafetyCheckFailed, "Unknown delivery journal version.")
	}
	switch journal.Phase {
	case deliveryPhasePrepared,
		deliveryPhaseCommitting,
		deliveryPhaseCommitted,
		deliveryPhasePushing,
		string(ExitPhasePR),
		exitStepCompleted,
		exitOperationCanceled,
		exitStepFailed:
		return &journal, nil
	default:
		return nil, refusal(ErrSafetyCheckFailed, "Unknown delivery journal phase.")
	}
}

func saveDeliveryJournal(path string, journal *managedDeliveryJournal) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(journal)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".delivery-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if _, err = file.Write(data); err != nil {
		file.Close()
		return err
	}
	if err = file.Sync(); err != nil {
		file.Close()
		return err
	}
	if err := file.Close(); err != nil {
		return err
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	return errors.Join(directory.Sync(), directory.Close())
}

func (s *Service) deliveryGitValue(ctx context.Context, item Worktree, args ...string) (string, error) {
	output, stderr, err := s.runner.Run(ctx, item.Path, args...)
	if err != nil {
		return "", refusal(ErrSafetyCheckFailed, exitCommandOutput(output, stderr, err))
	}
	return strings.TrimSpace(string(output)), nil
}

func (s *Service) deliveryIntentSnapshot(ctx context.Context, item Worktree, paths []string) (string, error) {
	// The expected tree depends on HEAD and selected content only. Unrelated
	// working files and staged entries cannot enter the delivery commit.
	return s.deliveryExpectedTree(ctx, item, paths)
}
