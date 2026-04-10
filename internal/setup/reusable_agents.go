package setup

import (
	"fmt"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/bundledagents"
	"github.com/compozy/compozy/internal/core/frontmatter"
)

const reusableAgentsInstallDir = ".compozy/agents"

// ListReusableAgents enumerates bundled reusable agents from the provided bundle.
func ListReusableAgents(bundle fs.FS) ([]ReusableAgent, error) {
	if bundle == nil {
		return nil, fmt.Errorf("list bundled reusable agents: bundle is nil")
	}

	entries, err := fs.ReadDir(bundle, ".")
	if err != nil {
		return nil, fmt.Errorf("list bundled reusable agents: %w", err)
	}

	reusableAgents := make([]ReusableAgent, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		reusableAgent, err := parseReusableAgent(bundle, entry.Name())
		if err != nil {
			return nil, err
		}
		reusableAgents = append(reusableAgents, reusableAgent)
	}

	slices.SortFunc(reusableAgents, func(left, right ReusableAgent) int {
		return strings.Compare(left.Name, right.Name)
	})
	return reusableAgents, nil
}

func parseReusableAgent(bundle fs.FS, dir string) (ReusableAgent, error) {
	agentPath := path.Join(dir, "AGENT.md")
	content, err := fs.ReadFile(bundle, agentPath)
	if err != nil {
		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: %w", dir, err)
	}

	var metadata struct {
		Title       string `yaml:"title"`
		Description string `yaml:"description"`
	}
	if _, err := frontmatter.Parse(string(content), &metadata); err != nil {
		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: %w", dir, err)
	}
	if strings.TrimSpace(metadata.Title) == "" || strings.TrimSpace(metadata.Description) == "" {
		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: missing title or description", dir)
	}

	return ReusableAgent{
		Name:        dir,
		Title:       strings.TrimSpace(metadata.Title),
		Description: strings.TrimSpace(metadata.Description),
		Directory:   dir,
	}, nil
}

// ListBundledReusableAgents returns the reusable agents bundled into the compozy binary.
func ListBundledReusableAgents() ([]ReusableAgent, error) {
	return ListReusableAgents(bundledagents.FS)
}

// PreviewBundledReusableAgentInstall resolves the on-disk install plan for bundled reusable agents.
func PreviewBundledReusableAgentInstall(options ResolverOptions) ([]ReusableAgentPreviewItem, error) {
	env, err := resolveEnvironment(options)
	if err != nil {
		return nil, err
	}

	reusableAgents, err := ListBundledReusableAgents()
	if err != nil {
		return nil, err
	}

	items := make([]ReusableAgentPreviewItem, 0, len(reusableAgents))
	root := filepath.Join(env.homeDir, reusableAgentsInstallDir)
	for _, reusableAgent := range reusableAgents {
		targetPath := filepath.Join(root, reusableAgent.Name)
		items = append(items, ReusableAgentPreviewItem{
			ReusableAgent: reusableAgent,
			TargetPath:    targetPath,
			WillOverwrite: pathExists(targetPath),
		})
	}
	return items, nil
}

// InstallBundledReusableAgents installs every bundled reusable agent into the canonical global root.
func InstallBundledReusableAgents(
	options ResolverOptions,
) ([]ReusableAgentSuccessItem, []ReusableAgentFailureItem, error) {
	env, err := resolveEnvironment(options)
	if err != nil {
		return nil, nil, err
	}

	reusableAgents, err := ListBundledReusableAgents()
	if err != nil {
		return nil, nil, err
	}

	successes := make([]ReusableAgentSuccessItem, 0, len(reusableAgents))
	failures := make([]ReusableAgentFailureItem, 0)
	root := filepath.Join(env.homeDir, reusableAgentsInstallDir)
	for _, reusableAgent := range reusableAgents {
		targetPath := filepath.Join(root, reusableAgent.Name)
		if err := cleanAndCreateDirectory(targetPath); err != nil {
			failures = append(failures, ReusableAgentFailureItem{
				ReusableAgent: reusableAgent,
				Path:          targetPath,
				Error:         err.Error(),
			})
			continue
		}
		if err := copyBundleDirectory(
			bundledagents.FS,
			reusableAgent.Directory,
			targetPath,
			"bundled reusable agent",
		); err != nil {
			failures = append(failures, ReusableAgentFailureItem{
				ReusableAgent: reusableAgent,
				Path:          targetPath,
				Error:         err.Error(),
			})
			continue
		}

		successes = append(successes, ReusableAgentSuccessItem{
			ReusableAgent: reusableAgent,
			Path:          targetPath,
		})
	}

	return successes, failures, nil
}
