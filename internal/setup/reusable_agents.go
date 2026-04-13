package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/agents"
	"github.com/compozy/compozy/internal/core/frontmatter"
)

const reusableAgentsInstallDir = ".compozy/agents"

var (
	copyReusableAgentBundleDirectory = copyBundleDirectory
	createReusableAgentTempDir       = os.MkdirTemp
	removeReusableAgentPath          = os.RemoveAll
	renameReusableAgentPath          = os.Rename
)

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
	if err := validateReusableAgentName(dir); err != nil {
		return ReusableAgent{}, fmt.Errorf("read bundled reusable agent %q: %w", dir, err)
	}

	return ReusableAgent{
		Name:        dir,
		Title:       strings.TrimSpace(metadata.Title),
		Description: strings.TrimSpace(metadata.Description),
		Directory:   dir,
		Origin:      AssetOriginBundled,
		SourceFS:    bundle,
		SourceDir:   dir,
	}, nil
}

func validateReusableAgentName(name string) error {
	trimmed := strings.TrimSpace(name)
	switch {
	case trimmed == "":
		return fmt.Errorf("reusable agent name is required")
	case trimmed == ".":
		return fmt.Errorf("reusable agent name %q must not resolve to the current directory", name)
	case strings.Contains(trimmed, ".."):
		return fmt.Errorf("reusable agent name %q must not contain \"..\"", name)
	case strings.ContainsAny(trimmed, `/\`):
		return fmt.Errorf("reusable agent name %q must not contain path separators", name)
	}

	base := filepath.Base(trimmed)
	if base == "." || base == "" {
		return fmt.Errorf("reusable agent name %q is invalid", name)
	}
	return nil
}

// ListBundledReusableAgents returns the reusable agents bundled into the compozy binary.
func ListBundledReusableAgents() ([]ReusableAgent, error) {
	return ListReusableAgents(agents.FS)
}

// PreviewBundledReusableAgentInstall resolves the on-disk install plan for bundled reusable agents.
func PreviewBundledReusableAgentInstall(cfg ReusableAgentInstallConfig) ([]ReusableAgentPreviewItem, error) {
	reusableAgents, err := ListBundledReusableAgents()
	if err != nil {
		return nil, err
	}
	cfg.ReusableAgents = reusableAgents
	return PreviewReusableAgentInstall(cfg)
}

// InstallBundledReusableAgents installs every bundled reusable agent into the selected setup scope.
func InstallBundledReusableAgents(
	cfg ReusableAgentInstallConfig,
) ([]ReusableAgentSuccessItem, []ReusableAgentFailureItem, error) {
	reusableAgents, err := ListBundledReusableAgents()
	if err != nil {
		return nil, nil, err
	}
	cfg.ReusableAgents = reusableAgents
	return InstallReusableAgents(cfg)
}

func prepareReusableAgentInstallTarget(root, name string) (string, error) {
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", fmt.Errorf("prepare reusable agent install root %s: %w", root, err)
	}
	tempTarget, err := createReusableAgentTempDir(root, name+".tmp-*")
	if err != nil {
		return "", fmt.Errorf("prepare reusable agent staging directory for %q: %w", name, err)
	}
	return tempTarget, nil
}

func replaceReusableAgentInstallTarget(tempTarget, targetPath string) error {
	backupPath := ""
	if pathExists(targetPath) {
		backupPath = filepath.Join(
			filepath.Dir(targetPath),
			fmt.Sprintf(".%s.backup-%d", filepath.Base(targetPath), os.Getpid()),
		)
		if err := removeReusableAgentPath(backupPath); err != nil {
			return fmt.Errorf("prepare reusable agent backup %s: %w", backupPath, err)
		}
		if err := renameReusableAgentPath(targetPath, backupPath); err != nil {
			return fmt.Errorf("replace reusable agent install %s: %w", targetPath, err)
		}
	}
	if err := renameReusableAgentPath(tempTarget, targetPath); err != nil {
		rollbackErr := error(nil)
		if backupPath != "" {
			rollbackErr = renameReusableAgentPath(backupPath, targetPath)
		}
		if rollbackErr != nil {
			return errors.Join(
				fmt.Errorf("replace reusable agent install %s: %w", targetPath, err),
				fmt.Errorf("restore reusable agent backup %s: %w", backupPath, rollbackErr),
			)
		}
		return fmt.Errorf("replace reusable agent install %s: %w", targetPath, err)
	}
	if backupPath != "" {
		if err := removeReusableAgentPath(backupPath); err != nil {
			return fmt.Errorf("cleanup reusable agent backup %s: %w", backupPath, err)
		}
	}
	return nil
}
