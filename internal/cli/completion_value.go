package cli

import (
	"cmp"
	"slices"
	"strings"

	"github.com/spf13/cobra"
)

type completionKind string

const (
	kindWorkspace       completionKind = "workspace"
	kindProfile         completionKind = "profile"
	kindAgent           completionKind = "agent"
	kindProvider        completionKind = "provider"
	kindModel           completionKind = "model"
	kindSession         completionKind = "session"
	kindTask            completionKind = "task"
	kindLoop            completionKind = "loop"
	kindAutomation      completionKind = "automation"
	kindWorktree        completionKind = "worktree"
	kindSessionFolder   completionKind = "session_folder"
	kindWorkspaceFolder completionKind = "workspace_folder"
	kindExtension       completionKind = "extension"
	kindComputer        completionKind = "computer"
	kindVault           completionKind = "vault"
	kindToolset         completionKind = "toolset"
	kindOutput          completionKind = "output"
)

type completionCandidate struct {
	value       string
	description string
}

func (c completionCandidate) render() string {
	value := sanitizeCompletionField(c.value)
	if value == "" {
		return ""
	}
	description := sanitizeCompletionField(c.description)
	if description == "" {
		return value
	}
	return cobra.CompletionWithDesc(value, description)
}

func sanitizeCompletionField(raw string) string {
	replacer := strings.NewReplacer("\t", " ", "\r", " ", "\n", " ")
	return strings.TrimSpace(replacer.Replace(raw))
}

func prepareCompletionCandidates(candidates []completionCandidate, prefix string) []completionCandidate {
	if len(candidates) == 0 {
		return nil
	}
	seen := make(map[string]int, len(candidates))
	filtered := make([]completionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		value := sanitizeCompletionField(candidate.value)
		if value == "" || !strings.HasPrefix(value, prefix) {
			continue
		}
		description := sanitizeCompletionField(candidate.description)
		if index, exists := seen[value]; exists {
			if filtered[index].description == "" && description != "" {
				filtered[index].description = description
			}
			continue
		}
		seen[value] = len(filtered)
		filtered = append(filtered, completionCandidate{
			value:       value,
			description: description,
		})
	}
	if len(filtered) == 0 {
		return nil
	}
	slices.SortStableFunc(filtered, func(a, b completionCandidate) int {
		return cmp.Compare(a.value, b.value)
	})
	return filtered
}

func renderCompletionCandidates(candidates []completionCandidate, prefix string) []cobra.Completion {
	prepared := prepareCompletionCandidates(candidates, prefix)
	if len(prepared) == 0 {
		return nil
	}
	rendered := make([]cobra.Completion, 0, len(prepared))
	for _, candidate := range prepared {
		if line := candidate.render(); line != "" {
			rendered = append(rendered, line)
		}
	}
	if len(rendered) == 0 {
		return nil
	}
	return rendered
}

func completionAdapter(
	directive cobra.ShellCompDirective,
	fetch func(cmd *cobra.Command, args []string, toComplete string) []completionCandidate,
) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
		if fetch == nil {
			return nil, directive
		}
		candidates := fetch(cmd, args, toComplete)
		return renderCompletionCandidates(candidates, toComplete), directive
	}
}
