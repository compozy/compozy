package extractor

import (
	"bufio"

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"strings"
	"time"

	"github.com/compozy/agh/internal/fileutil"
	memcontract "github.com/compozy/agh/internal/memory/contract"
)

type pendingFile struct {
	path    string
	modTime time.Time
	claimed bool
}

func (c *InboxConsumer) pendingFiles() ([]pendingFile, error) {
	sessionDirs, err := os.ReadDir(c.inboxRoot)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []pendingFile{}, nil
		}
		return nil, fmt.Errorf("memory extractor: read inbox root: %w", err)
	}
	files := make([]pendingFile, 0)
	for _, sessionDir := range sessionDirs {
		if !sessionDir.IsDir() {
			continue
		}
		dirPath := filepath.Join(c.inboxRoot, sessionDir.Name())
		entries, err := os.ReadDir(dirPath)
		if err != nil {
			return nil, fmt.Errorf("memory extractor: read inbox session %q: %w", sessionDir.Name(), err)
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			claimed := false
			switch {
			case strings.HasSuffix(entry.Name(), ".jsonl"):
			case strings.HasSuffix(entry.Name(), ".jsonl"+processingSuffix):
				claimed = true
			default:
				continue
			}
			info, err := entry.Info()
			if err != nil {
				return nil, fmt.Errorf("memory extractor: stat inbox file %q: %w", entry.Name(), err)
			}
			files = append(files, pendingFile{
				path:    filepath.Join(dirPath, entry.Name()),
				modTime: info.ModTime().UTC(),
				claimed: claimed,
			})
		}
	}
	slices.SortFunc(files, func(a, b pendingFile) int {
		if !a.modTime.Equal(b.modTime) {
			return a.modTime.Compare(b.modTime)
		}
		return strings.Compare(a.path, b.path)
	})
	return files, nil
}

func (c *InboxConsumer) consumeFile(ctx context.Context, file pendingFile) (ConsumeResult, error) {
	result := ConsumeResult{Decisions: make([]memcontract.Decision, 0), Failures: make([]string, 0)}
	path := file.path
	processing := path + processingSuffix
	if file.claimed {
		processing = file.path
		path = strings.TrimSuffix(file.path, processingSuffix)
	} else if err := os.Rename(path, processing); err != nil {
		return result, fmt.Errorf("memory extractor: claim inbox file %q: %w", path, err)
	}

	candidates, err := decodeCandidateFile(processing)
	if err != nil {
		result.Failed++
		failurePath, moveErr := c.moveToDLQ(processing, "decode", err)
		result.Failures = append(result.Failures, failurePath)
		c.recordFailure(ctx, "", failurePath, "decode", err)
		return result, errors.Join(err, moveErr)
	}

	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return result, c.requeueClaimedFile(processing, path, err)
		}
		decision, err := c.sink.ProposeCandidate(ctx, candidate)
		if err != nil {
			if isRetryableInboxError(ctx, err) {
				return result, c.requeueClaimedFile(processing, path, err)
			}
			result.Failed++
			failurePath, moveErr := c.moveToDLQ(processing, "controller", err)
			result.Failures = append(result.Failures, failurePath)
			c.recordFailure(ctx, candidate.Metadata["session_id"], failurePath, "controller", err)
			return result, errors.Join(
				fmt.Errorf("memory extractor: propose candidate from %q: %w", path, err),
				moveErr,
			)
		}
		result.Proposed++
		result.Decisions = append(result.Decisions, decision)
	}

	if err := fileutil.AtomicRemoveFile(processing); err != nil {
		return result, fmt.Errorf("memory extractor: remove consumed inbox file: %w", err)
	}
	return result, nil
}

func (c *InboxConsumer) requeueClaimedFile(processing string, path string, cause error) error {
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("memory extractor: requeue inbox file %q: target already exists: %w", path, cause)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("memory extractor: stat inbox requeue target %q: %w", path, err)
	}
	if err := os.Rename(processing, path); err != nil {
		return errors.Join(
			fmt.Errorf("memory extractor: requeue inbox file %q: %w", path, err),
			cause,
		)
	}
	return fmt.Errorf("memory extractor: consume interrupted for %q: %w", path, cause)
}

func isRetryableInboxError(ctx context.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	if ctx == nil {
		return false
	}
	ctxErr := ctx.Err()
	return ctxErr != nil && errors.Is(err, ctxErr)
}

func decodeCandidateFile(path string) ([]memcontract.Candidate, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("memory extractor: open candidate file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			slog.Default().Warn("memory extractor: close candidate file failed", "path", path, "error", closeErr)
		}
	}()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), defaultScannerLimit)
	candidates := make([]memcontract.Candidate, 0)
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		var candidate memcontract.Candidate
		if err := json.Unmarshal([]byte(raw), &candidate); err != nil {
			return nil, fmt.Errorf("memory extractor: decode candidate line %d: %w", line, err)
		}
		candidates = append(candidates, candidate)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("memory extractor: scan candidate file: %w", err)
	}
	return candidates, nil
}
