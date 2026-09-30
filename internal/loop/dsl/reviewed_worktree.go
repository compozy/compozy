package dsl

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

const ReviewedWorktreeInput = "reviewed_worktree"
const maxReviewedWorktreePaths = 4096
const maxReviewedWorktreeProofBytes = 1 << 20

type ReviewedWorktreeCandidate struct {
	WorktreeID   string   `json:"worktree_id"   yaml:"worktree_id"`
	IncludePaths []string `json:"include_paths" yaml:"include_paths"`
	Fingerprint  string   `json:"fingerprint"   yaml:"fingerprint"`
}

func DecodeReviewedWorktree(inputs map[string]any) (*ReviewedWorktreeCandidate, error) {
	raw, exists := inputs[ReviewedWorktreeInput]
	if !exists || raw == nil {
		return nil, nil
	}
	serialized, ok := raw.(string)
	if !ok {
		return nil, errors.New("reviewed_worktree must be a JSON string")
	}
	if len(serialized) > maxReviewedWorktreeProofBytes {
		return nil, errors.New("reviewed_worktree JSON string exceeds 1 MiB")
	}
	data := []byte(serialized)
	var candidate ReviewedWorktreeCandidate
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&candidate); err != nil {
		return nil, fmt.Errorf("reviewed_worktree: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("reviewed_worktree must contain exactly one JSON object")
	}
	if candidate.WorktreeID == "" || strings.TrimSpace(candidate.WorktreeID) != candidate.WorktreeID ||
		len(candidate.WorktreeID) > 256 {
		return nil, errors.New("reviewed_worktree.worktree_id must be a registered worktree ID")
	}
	digest, err := hex.DecodeString(candidate.Fingerprint)
	if err != nil || len(digest) != 32 {
		return nil, errors.New("reviewed_worktree.fingerprint must be the scoped exit plan SHA-256 fingerprint")
	}
	if len(candidate.IncludePaths) == 0 || len(candidate.IncludePaths) > maxReviewedWorktreePaths {
		return nil, errors.New("reviewed_worktree.include_paths must contain between 1 and 4096 exact file paths")
	}
	if err := validateReviewedWorktreePaths(candidate.IncludePaths); err != nil {
		return nil, err
	}
	return &candidate, nil
}

func validateReviewedWorktreePaths(paths []string) error {
	for _, path := range paths {
		if path == "" || strings.TrimSpace(path) == "" || len(path) > 4096 || filepath.IsAbs(path) ||
			filepath.Clean(path) != path ||
			path == "." ||
			path == ".." ||
			strings.HasPrefix(path, "../") ||
			strings.Contains(path, "\\") ||
			strings.ContainsRune(path, 0) {
			return errors.New("reviewed_worktree.include_paths contains an invalid file path")
		}
	}
	return nil
}
