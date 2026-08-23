package demoseed

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/config"
)

const memoryIndexFile = "MEMORY.md"

func seedMemories(state *scenario) (int, error) {
	stories := scenarioMemories(state.clock)
	byDir := map[string][]memoryStory{}
	for _, story := range stories {
		dir, err := memoryDirFor(state, story)
		if err != nil {
			return 0, err
		}
		byDir[dir] = append(byDir[dir], story)
	}
	for dir, scoped := range byDir {
		if err := writeMemoryScope(dir, scoped); err != nil {
			return 0, err
		}
	}
	return len(stories), nil
}

func memoryDirFor(state *scenario, story memoryStory) (string, error) {
	switch story.Scope {
	case memoryScopeGlobal:
		return state.paths.MemoryDir, nil
	case memoryScopeWorkspace:
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return "", err
		}
		return filepath.Join(record.RootDir, config.DirName, config.MemoryDirName), nil
	case memoryScopeAgent:
		record, err := state.recordFor(story.WorkspaceKey)
		if err != nil {
			return "", err
		}
		return filepath.Join(
			record.RootDir, config.DirName, config.AgentsDirName, story.AgentName, config.MemoryDirName,
		), nil
	default:
		return "", fmt.Errorf("demo seed: unknown memory scope %q", story.Scope)
	}
}

func writeMemoryScope(dir string, stories []memoryStory) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("demo seed: create memory directory %q: %w", dir, err)
	}
	sort.Slice(stories, func(left int, right int) bool {
		if stories[left].UpdatedAt.Equal(stories[right].UpdatedAt) {
			return stories[left].Name < stories[right].Name
		}
		return stories[left].UpdatedAt.After(stories[right].UpdatedAt)
	})
	index := make([]string, 0, len(stories))
	for _, story := range stories {
		path := filepath.Join(dir, story.Name+".md")
		if err := os.WriteFile(path, []byte(renderMemoryFile(story)), 0o600); err != nil {
			return fmt.Errorf("demo seed: write memory %q: %w", story.Name, err)
		}
		if err := os.Chtimes(path, story.UpdatedAt, story.UpdatedAt); err != nil {
			return fmt.Errorf("demo seed: set memory time %q: %w", story.Name, err)
		}
		index = append(index, fmt.Sprintf("- [%s](%s.md) - %s", story.Name, story.Name, story.Description))
	}
	indexPath := filepath.Join(dir, memoryIndexFile)
	if err := os.WriteFile(indexPath, []byte(strings.Join(index, "\n")+"\n"), 0o600); err != nil {
		return fmt.Errorf("demo seed: write memory index %q: %w", indexPath, err)
	}
	return nil
}

func renderMemoryFile(story memoryStory) string {
	var builder strings.Builder
	builder.WriteString("---\n")
	fmt.Fprintf(&builder, "name: %s\n", strconv.Quote(story.Name))
	fmt.Fprintf(&builder, "description: %s\n", strconv.Quote(story.Description))
	fmt.Fprintf(&builder, "type: %s\n", strconv.Quote(story.Type))
	fmt.Fprintf(&builder, "scope: %s\n", strconv.Quote(story.Scope))
	if story.Scope == memoryScopeAgent {
		fmt.Fprintf(&builder, "agent: %s\n", strconv.Quote(story.AgentName))
		builder.WriteString("agent_tier: workspace\n")
	}
	builder.WriteString("---\n\n")
	builder.WriteString(story.Body)
	builder.WriteString("\n")
	return builder.String()
}
