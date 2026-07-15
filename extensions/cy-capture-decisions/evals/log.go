package evals

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/extensions/cy-capture-decisions/decisionlog"
	"github.com/compozy/compozy/internal/core/frontmatter"
)

type logSnapshot struct {
	Records map[string]decisionlog.DecisionRecordMeta
	Indexed map[string]struct{}
}

func loadLog(workspaceRoot string) (logSnapshot, error) {
	root := filepath.Join(workspaceRoot, ".compozy")
	if err := decisionlog.Validate(os.DirFS(root)); err != nil {
		return logSnapshot{}, err
	}
	snapshot := logSnapshot{
		Records: make(map[string]decisionlog.DecisionRecordMeta),
		Indexed: make(map[string]struct{}),
	}
	entries, err := os.ReadDir(filepath.Join(root, "decisions"))
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return logSnapshot{}, fmt.Errorf("read decision bodies: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "AD-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(root, "decisions", entry.Name()))
		if err != nil {
			return logSnapshot{}, err
		}
		var meta decisionlog.DecisionRecordMeta
		if _, err := frontmatter.Parse(string(content), &meta); err != nil {
			return logSnapshot{}, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		snapshot.Records[meta.ID] = meta
	}
	index, err := os.ReadFile(filepath.Join(root, "DECISIONS.md"))
	if err != nil {
		return logSnapshot{}, err
	}
	for _, line := range strings.Split(string(index), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		fields := strings.Split(trimmed, "|")
		if len(fields) > 0 {
			snapshot.Indexed[strings.TrimSpace(fields[0])] = struct{}{}
		}
	}
	return snapshot, nil
}

func findByProvenance(snapshot logSnapshot, slug, sourceADR string) (decisionlog.DecisionRecordMeta, error) {
	for id := range snapshot.Records {
		record := snapshot.Records[id]
		if record.SourceSlug == slug && record.SourceADR == sourceADR {
			return record, nil
		}
	}
	return decisionlog.DecisionRecordMeta{}, fmt.Errorf("record not found for %s/%s", slug, sourceADR)
}

func requireStatus(snapshot logSnapshot, slug, sourceADR, status string) (decisionlog.DecisionRecordMeta, error) {
	record, err := findByProvenance(snapshot, slug, sourceADR)
	if err != nil {
		return decisionlog.DecisionRecordMeta{}, err
	}
	if record.Status != status {
		return decisionlog.DecisionRecordMeta{}, fmt.Errorf("%s status = %q, want %q", record.ID, record.Status, status)
	}
	return record, nil
}

func requireIndexed(snapshot logSnapshot, id string, want bool) error {
	_, indexed := snapshot.Indexed[id]
	if indexed != want {
		return fmt.Errorf("index membership for %s = %t, want %t", id, indexed, want)
	}
	return nil
}

func requireContains(content, needle, label string) error {
	if !strings.Contains(strings.ToLower(content), strings.ToLower(needle)) {
		return fmt.Errorf("%s does not contain %q", label, needle)
	}
	return nil
}

func requireNotContains(content, needle, label string) error {
	if strings.Contains(strings.ToLower(content), strings.ToLower(needle)) {
		return fmt.Errorf("%s unexpectedly contains %q", label, needle)
	}
	return nil
}
