package extractor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"os"
	"path/filepath"

	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/fileutil"
)

func (c *InboxConsumer) moveToDLQ(path string, stage string, cause error) (string, error) {
	if err := os.MkdirAll(c.failuresDir, inboxDirPerm); err != nil {
		return "", fmt.Errorf("memory extractor: ensure dlq dir: %w", err)
	}
	content, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", fmt.Errorf("memory extractor: read failed inbox file: %w", readErr)
	}
	report := map[string]string{
		"stage":       strings.TrimSpace(stage),
		"source":      path,
		"error":       cause.Error(),
		"content":     string(content),
		"recorded_at": c.now().UTC().Format(time.RFC3339Nano),
	}
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("memory extractor: encode dlq report: %w", err)
	}
	target := filepath.Join(c.failuresDir, dlqFilename(c.now(), filepath.Base(path)))
	if err := fileutil.AtomicWriteFile(target, append(encoded, '\n'), inboxFilePerm); err != nil {
		return "", fmt.Errorf("memory extractor: write dlq report: %w", err)
	}
	if err := fileutil.AtomicRemoveFile(path); err != nil {
		return target, fmt.Errorf("memory extractor: remove failed inbox file: %w", err)
	}
	return target, nil
}

func (c *InboxConsumer) recordFailure(
	ctx context.Context,
	sessionID string,
	target string,
	stage string,
	cause error,
) {
	if c.events == nil {
		return
	}
	event := Event{
		Op:        EventFailed,
		SessionID: sessionID,
		TargetID:  target,
		Error:     cause.Error(),
		Metadata: map[string]string{
			"stage": strings.TrimSpace(stage),
		},
		At: c.now().UTC(),
	}
	if err := c.events.RecordExtractorEvent(ctx, event); err != nil {
		c.logger.Warn("memory extractor: record failure event failed", "error", err)
	}
}

func cleanRoot(root string) (string, error) {
	clean := strings.TrimSpace(root)
	if clean == "" {
		return "", errors.New("memory extractor: memory root is required")
	}
	return filepath.Clean(clean), nil
}

func inboxSessionDir(inboxRoot string, sessionID string) (string, error) {
	segment, err := safeSegment(sessionID)
	if err != nil {
		return "", err
	}
	return filepath.Join(inboxRoot, segment), nil
}

func safeSegment(raw string) (string, error) {
	segment := strings.TrimSpace(raw)
	if segment == "" {
		return "", errors.New("memory extractor: path segment is required")
	}
	if strings.Contains(segment, "/") || strings.Contains(segment, `\`) || segment == "." || segment == ".." {
		return "", fmt.Errorf("memory extractor: unsafe path segment %q", raw)
	}
	return segment, nil
}

func inboxFilename(now time.Time, seq int64) string {
	return now.UTC().Format("20060102T150405.000000000Z") + "-" + strconv.FormatInt(seq, 10) + ".jsonl"
}

func dlqFilename(now time.Time, base string) string {
	cleanBase := strings.TrimSpace(base)
	if cleanBase == "" {
		cleanBase = "inbox.jsonl"
	}
	return now.UTC().Format("20060102T150405.000000000Z") + "-" + cleanBase + ".json"
}
