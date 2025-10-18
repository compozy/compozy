package project

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/engine/core"
)

type WorkflowsValidator struct {
	cwd       *core.PathCWD
	workflows []*WorkflowSourceConfig
	statFn    func(string) (fs.FileInfo, error)
	statCache map[string]fs.FileInfo
}

func NewWorkflowsValidator(cwd *core.PathCWD, workflows []*WorkflowSourceConfig) *WorkflowsValidator {
	return &WorkflowsValidator{
		cwd:       cwd,
		workflows: workflows,
		statFn:    os.Stat,
	}
}

func (v *WorkflowsValidator) Validate(_ context.Context) error {
	if len(v.workflows) == 0 {
		return nil
	}
	if v.statFn == nil {
		v.statFn = os.Stat
	}
	v.statCache = make(map[string]fs.FileInfo, len(v.workflows))
	seen := make(map[string]int, len(v.workflows))
	for idx, wf := range v.workflows {
		if wf == nil {
			return fmt.Errorf("workflow[%d] configuration is nil", idx)
		}
		source := strings.TrimSpace(wf.Source)
		if source == "" {
			return fmt.Errorf("workflow[%d] source is empty", idx)
		}
		resolvedPath, err := v.resolveWorkflowPath(source)
		if err != nil {
			return fmt.Errorf("workflow[%d] source '%s': %w", idx, source, err)
		}
		if prevIdx, ok := seen[resolvedPath]; ok {
			return fmt.Errorf(
				"workflow[%d] source '%s' duplicates workflow[%d] source '%s'",
				idx,
				source,
				prevIdx,
				v.workflows[prevIdx].Source,
			)
		}
		seen[resolvedPath] = idx
		info, err := v.statPath(resolvedPath)
		if err != nil {
			return fmt.Errorf("workflow[%d] source '%s': %w", idx, source, err)
		}
		if info.IsDir() {
			return fmt.Errorf("workflow[%d] source '%s' points to a directory", idx, source)
		}
	}
	return nil
}

func (v *WorkflowsValidator) resolveWorkflowPath(source string) (string, error) {
	cleaned := filepath.Clean(source)
	if filepath.IsAbs(cleaned) {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("failed to resolve absolute path: %w", err)
		}
		return abs, nil
	}
	base := ""
	if v.cwd != nil {
		base = strings.TrimSpace(v.cwd.PathStr())
	}
	if base == "" {
		abs, err := filepath.Abs(cleaned)
		if err != nil {
			return "", fmt.Errorf("failed to resolve relative path: %w", err)
		}
		return abs, nil
	}
	return filepath.Join(base, cleaned), nil
}

func (v *WorkflowsValidator) statPath(path string) (fs.FileInfo, error) {
	if info, ok := v.statCache[path]; ok {
		return info, nil
	}
	info, err := v.statFn(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("workflow file '%s' not found", path)
		}
		return nil, fmt.Errorf("failed to stat workflow file '%s': %w", path, err)
	}
	v.statCache[path] = info
	return info, nil
}

// -----------------------------------------------------------------------------
// WebhookSlugsValidator - Validates uniqueness of webhook slugs across workflows
// -----------------------------------------------------------------------------

type WebhookSlugsValidator struct {
	slugs []string
}

func NewWebhookSlugsValidator(slugs []string) *WebhookSlugsValidator {
	return &WebhookSlugsValidator{slugs: slugs}
}

func (v *WebhookSlugsValidator) Validate(_ context.Context) error {
	seen := make(map[string]struct{}, len(v.slugs))
	for _, slug := range v.slugs {
		if slug == "" {
			continue
		}
		if _, ok := seen[slug]; ok {
			return fmt.Errorf("duplicate webhook slug '%s'", slug)
		}
		seen[slug] = struct{}{}
	}
	return nil
}
