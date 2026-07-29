package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/skills"
	skillmarketplace "github.com/compozy/compozy/internal/skills/marketplace"
)

func createWorkspaceSkill(
	skillsRoot string,
	group string,
	skillName string,
) (skillDir string, skillFile string, err error) {
	if err := os.MkdirAll(skillsRoot, 0o755); err != nil {
		return "", "", fmt.Errorf("cli: create skills directory %q: %w", skillsRoot, err)
	}
	rootInfo, err := os.Lstat(skillsRoot)
	if err != nil {
		return "", "", fmt.Errorf("cli: inspect skills directory %q: %w", skillsRoot, err)
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 {
		return "", "", fmt.Errorf("cli: skills directory %q must not be a symlink", skillsRoot)
	}
	if _, err := skillmarketplace.PathInsideRoot(skillsRoot, skillsRoot); err != nil {
		return "", "", fmt.Errorf("cli: validate skills directory %q: %w", skillsRoot, err)
	}

	groupDir, createdGroups, err := createSkillGroupDirectories(skillsRoot, group)
	if err != nil {
		return "", "", err
	}
	committed := false
	stagingDir := ""
	defer func() {
		if committed {
			return
		}
		cleanupErr := cleanupSkillCreate(stagingDir, createdGroups)
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()

	skillDir = filepath.Join(groupDir, skillName)
	if _, err := skillmarketplace.PathInsideRoot(skillsRoot, skillDir); err != nil {
		return "", "", fmt.Errorf("cli: validate skill directory %q: %w", skillDir, err)
	}
	if _, err := os.Lstat(skillDir); err == nil {
		return "", "", fmt.Errorf("skill %q already exists at %s", skillName, skillDir)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", "", fmt.Errorf("cli: inspect skill directory %q: %w", skillDir, err)
	}

	stagingDir, err = os.MkdirTemp(skillsRoot, ".compozy-skill-create-*")
	if err != nil {
		return "", "", fmt.Errorf("cli: create skill staging directory: %w", err)
	}
	stagingFile := filepath.Join(stagingDir, skillMarkdownFileName)
	if err := os.WriteFile(stagingFile, []byte(defaultSkillTemplate(skillName)), 0o600); err != nil {
		return "", "", fmt.Errorf("cli: write skill template %q: %w", stagingFile, err)
	}
	if _, err := skills.ParseSkillFile(stagingFile); err != nil {
		return "", "", fmt.Errorf("cli: validate generated skill %q: %w", stagingFile, err)
	}
	if _, err := skillmarketplace.PathInsideRoot(skillsRoot, skillDir); err != nil {
		return "", "", fmt.Errorf("cli: validate skill directory %q before commit: %w", skillDir, err)
	}
	if err := os.Chmod(stagingDir, 0o755); err != nil {
		return "", "", fmt.Errorf("cli: set skill staging directory mode %q: %w", stagingDir, err)
	}
	if err := os.Rename(stagingDir, skillDir); err != nil {
		return "", "", fmt.Errorf("cli: commit skill directory %q: %w", skillDir, err)
	}

	committed = true
	return skillDir, filepath.Join(skillDir, skillMarkdownFileName), nil
}

func createSkillGroupDirectories(skillsRoot string, group string) (groupDir string, created []string, err error) {
	if group == "" {
		return skillsRoot, nil, nil
	}

	current := skillsRoot
	created = make([]string, 0, len(strings.Split(group, "/")))
	defer func() {
		if err == nil {
			return
		}
		cleanupErr := cleanupSkillCreate("", created)
		if cleanupErr != nil {
			err = errors.Join(err, cleanupErr)
		}
	}()
	for segment := range strings.SplitSeq(group, "/") {
		next := filepath.Join(current, segment)
		info, err := os.Lstat(next)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			if err := os.Mkdir(next, 0o755); err != nil {
				return "", created, fmt.Errorf("cli: create skill group directory %q: %w", next, err)
			}
			created = append(created, next)
		case err != nil:
			return "", created, fmt.Errorf("cli: inspect skill group directory %q: %w", next, err)
		case info.Mode()&fs.ModeSymlink != 0:
			return "", created, fmt.Errorf("cli: skill group directory %q must not be a symlink", next)
		case !info.IsDir():
			return "", created, fmt.Errorf("cli: skill group path %q is not a directory", next)
		}
		if _, err := skillmarketplace.PathInsideRoot(skillsRoot, next); err != nil {
			return "", created, fmt.Errorf("cli: validate skill group directory %q: %w", next, err)
		}
		current = next
	}
	return current, created, nil
}

func cleanupSkillCreate(stagingDir string, createdGroups []string) error {
	var cleanupErr error
	if stagingDir != "" {
		if err := os.RemoveAll(stagingDir); err != nil {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("cli: remove skill staging directory %q: %w", stagingDir, err),
			)
		}
	}
	for _, groupDir := range slices.Backward(createdGroups) {
		if err := os.Remove(groupDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanupErr = errors.Join(
				cleanupErr,
				fmt.Errorf("cli: remove empty skill group directory %q: %w", groupDir, err),
			)
		}
	}
	return cleanupErr
}
