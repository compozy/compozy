package cli

import (
	"fmt"
	"strings"

	"github.com/aymanbagabas/go-udiff"
	"github.com/compozy/compozy/internal/api/contract"
)

// Edit review change kinds and the review kind, as the daemon reports them.
const (
	skillReviewKindEdit  = "edit"
	skillChangeAdded     = "added"
	skillChangeRemoved   = "removed"
	skillChangeModified  = "modified"
	skillChangeUnchanged = "unchanged"
	skillDiffNoFile      = "/dev/null"
)

// agentSkillEditText renders an edit review's base counts, change counts and a unified diff of every
// added, removed or modified file. Binary base files show only their name and size.
func agentSkillEditText(text *strings.Builder, review contract.AgentSkillReviewPayload) {
	proposed, base := review.RunsAutomatically, review.BaseRunsAutomatically
	if base == nil {
		base = &contract.AgentSkillAutomaticContents{}
	}
	fmt.Fprintf(text, "Runs automatically: %d hooks, %d MCP servers (base: %d hooks, %d MCP servers)\n",
		proposed.Hooks, proposed.MCPServers, base.Hooks, base.MCPServers)
	fmt.Fprintf(text, "Supporting scripts: %d (base: %d)\n", proposed.Scripts, base.Scripts)
	if review.Problem != "" {
		fmt.Fprintf(text, "Problem: %s\n", review.Problem)
	}
	counts := map[string]int{}
	for _, change := range review.Changes {
		counts[change.Change]++
	}
	fmt.Fprintf(
		text,
		"Files: %d modified, %d added, %d removed\n",
		counts[skillChangeModified],
		counts[skillChangeAdded],
		counts[skillChangeRemoved],
	)
	files := make(map[string]string, len(review.Files))
	for _, file := range review.Files {
		files[file.Path] = file.Content
	}
	bases := make(map[string]contract.AgentSkillBaseFile, len(review.BaseFiles))
	for _, file := range review.BaseFiles {
		bases[file.Path] = file
	}
	for _, change := range review.Changes {
		if change.Change == skillChangeUnchanged {
			continue
		}
		before, after := bases[change.Path], files[change.Path]
		if before.Binary {
			fmt.Fprintf(text, "Binary file %s (%d bytes) %s\n", change.Path, before.Size, change.Change)
			continue
		}
		from, to := "a/"+change.Path, "b/"+change.Path
		switch change.Change {
		case skillChangeAdded:
			from = skillDiffNoFile
		case skillChangeRemoved:
			to, after = skillDiffNoFile, ""
		}
		text.WriteString(udiff.Unified(from, to, before.Content, after))
	}
}
