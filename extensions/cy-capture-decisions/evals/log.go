package evals

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
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
		return logSnapshot{}, fmt.Errorf("validate decision log in workspace %q: %w", workspaceRoot, err)
	}
	snapshot := logSnapshot{
		Records: make(map[string]decisionlog.DecisionRecordMeta),
		Indexed: make(map[string]struct{}),
	}
	decisionsDir := filepath.Join(root, "decisions")
	entries, err := os.ReadDir(decisionsDir)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return logSnapshot{}, fmt.Errorf("read decision directory %q: %w", decisionsDir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasPrefix(entry.Name(), "AD-") || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}
		path := filepath.Join(decisionsDir, entry.Name())
		content, err := os.ReadFile(path)
		if err != nil {
			return logSnapshot{}, fmt.Errorf("read decision body %q: %w", path, err)
		}
		var meta decisionlog.DecisionRecordMeta
		if _, err := frontmatter.Parse(string(content), &meta); err != nil {
			return logSnapshot{}, fmt.Errorf("parse %s: %w", entry.Name(), err)
		}
		snapshot.Records[meta.ID] = meta
	}
	indexPath := filepath.Join(root, "DECISIONS.md")
	index, err := os.ReadFile(indexPath)
	if err != nil {
		return logSnapshot{}, fmt.Errorf("read decision index %q: %w", indexPath, err)
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
	matches := make([]decisionlog.DecisionRecordMeta, 0, 1)
	for id := range snapshot.Records {
		record := snapshot.Records[id]
		if record.SourceSlug == slug && record.SourceADR == sourceADR {
			matches = append(matches, record)
		}
	}
	if len(matches) == 0 {
		return decisionlog.DecisionRecordMeta{}, fmt.Errorf("record not found for %s/%s", slug, sourceADR)
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	ids := make([]string, 0, len(matches))
	for i := range matches {
		ids = append(ids, matches[i].ID)
	}
	return decisionlog.DecisionRecordMeta{}, fmt.Errorf(
		"multiple records found for %s/%s: %s",
		slug,
		sourceADR,
		strings.Join(ids, ", "),
	)
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
