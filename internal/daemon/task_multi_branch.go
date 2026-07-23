package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/core/gitenv"
)

const taskMultiResultBranchRunSegmentLength = 8

// RenderedGroupContext contains the stable values available to one branch
// template expansion.
type RenderedGroupContext struct {
	ID        string
	Directory string
	Index     int
}

// BranchRenderInput contains the branch template and launch-specific token
// values for one parallel task-group batch.
type BranchRenderInput struct {
	Template   string
	Initiative string
	RunSegment string
	Groups     []RenderedGroupContext
}

type renderedResultBranch struct {
	groupID string
	name    string
}

// RenderResultBranches renders and validates one result branch per task group.
// It returns no partial map when any branch is invalid.
func RenderResultBranches(
	ctx context.Context,
	gitDir string,
	in BranchRenderInput,
) (map[string]string, bool, error) {
	if err := context.Cause(ctx); err != nil {
		return nil, false, fmt.Errorf("render result branches: %w", err)
	}

	runSegment := ""
	if strings.Contains(in.Template, "{run}") {
		var err error
		runSegment, err = renderTaskMultiRunSegment(in.RunSegment)
		if err != nil {
			return nil, false, err
		}
	}

	rendered := make([]renderedResultBranch, 0, len(in.Groups))
	seenGroupIDs := make(map[string]struct{}, len(in.Groups))
	for _, group := range in.Groups {
		groupID := strings.TrimSpace(group.ID)
		if groupID == "" {
			return nil, false, errors.New("render result branches: task group id is required")
		}
		if _, exists := seenGroupIDs[groupID]; exists {
			return nil, false, fmt.Errorf("render result branches: duplicate task group %s", groupID)
		}
		seenGroupIDs[groupID] = struct{}{}

		name := renderTaskMultiBranchTemplate(in, group, runSegment)
		if err := validateTaskMultiResultBranch(ctx, gitDir, groupID, name); err != nil {
			return nil, false, err
		}
		rendered = append(rendered, renderedResultBranch{groupID: groupID, name: name})
	}

	adjusted := !taskMultiResultBranchesUnique(rendered)
	if adjusted {
		for idx := range rendered {
			groupSegment := sanitizeTaskMultiWorktreeSegment(rendered[idx].groupID, taskMultiWorktreeSlugMaxLen)
			rendered[idx].name += "-" + groupSegment
			if err := validateTaskMultiResultBranch(
				ctx,
				gitDir,
				rendered[idx].groupID,
				rendered[idx].name,
			); err != nil {
				return nil, false, err
			}
		}
	}

	branches := make(map[string]string, len(rendered))
	for _, branch := range rendered {
		branches[branch.groupID] = branch.name
	}
	return branches, adjusted, nil
}

func renderTaskMultiBranchTemplate(
	in BranchRenderInput,
	group RenderedGroupContext,
	runSegment string,
) string {
	groupSegment := sanitizeTaskMultiWorktreeSegment(group.ID, taskMultiWorktreeSlugMaxLen)
	briefSegment := taskMultiGroupBriefSegment(group, groupSegment)
	indexSegment := sanitizeTaskMultiWorktreeSegment(
		fmt.Sprintf("%0*d", taskMultiWorktreeIndexPadWidth, group.Index),
		taskMultiWorktreeSlugMaxLen,
	)
	initiativeSegment := sanitizeTaskMultiWorktreeSegment(in.Initiative, taskMultiWorktreeSlugMaxLen)
	return strings.NewReplacer(
		"{initiative}", initiativeSegment,
		"{group_brief}", briefSegment,
		"{group}", groupSegment,
		"{index}", indexSegment,
		"{run}", runSegment,
	).Replace(in.Template)
}

func taskMultiGroupBriefSegment(group RenderedGroupContext, groupSegment string) string {
	directory := strings.TrimSpace(group.Directory)
	if directory == "" {
		return groupSegment
	}
	leaf := filepath.Base(filepath.Clean(directory))
	if leaf == "." || strings.EqualFold(leaf, strings.TrimSpace(group.ID)) {
		return groupSegment
	}
	brief := sanitizeTaskMultiWorktreeSegment(leaf, taskMultiWorktreeSlugMaxLen)
	if brief == "" {
		return groupSegment
	}
	return brief
}

func renderTaskMultiRunSegment(value string) (string, error) {
	raw := strings.TrimSpace(value)
	parts := strings.Split(raw, "-")
	for idx := len(parts) - 1; idx >= 0; idx-- {
		if strings.TrimSpace(parts[idx]) != "" {
			raw = parts[idx]
			break
		}
	}
	sanitized := sanitizeTaskMultiWorktreeSegment(raw, 0)
	if sanitized == "" {
		return "", errors.New("render result branches: run segment is required")
	}
	if len(sanitized) >= taskMultiResultBranchRunSegmentLength {
		return sanitized[:taskMultiResultBranchRunSegmentLength], nil
	}
	return sanitized + taskMultiShortHash(
		value,
		taskMultiResultBranchRunSegmentLength-len(sanitized),
	), nil
}

func validateTaskMultiResultBranch(ctx context.Context, gitDir string, groupID string, branch string) error {
	if _, err := gitenv.Run(ctx, gitDir, "check-ref-format", "--branch", branch); err != nil {
		return fmt.Errorf(
			"render result branch for %s: validate branch %q: %w",
			groupID,
			branch,
			err,
		)
	}
	return nil
}

func taskMultiResultBranchesUnique(branches []renderedResultBranch) bool {
	seen := make(map[string]struct{}, len(branches))
	for _, branch := range branches {
		if _, exists := seen[branch.name]; exists {
			return false
		}
		seen[branch.name] = struct{}{}
	}
	return true
}
