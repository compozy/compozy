package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	bundledskills "github.com/compozy/compozy/skills"
)

// PreviewSelectedSkills resolves the on-disk install plan for an explicit skill selection.
func PreviewSelectedSkills(
	options ResolverOptions,
	skills []Skill,
	agentNames []string,
	global bool,
	mode InstallMode,
) ([]PreviewItem, error) {
	selectedAgents, env, err := resolveSelectedAgentsForExtensionSkills(options, agentNames)
	if err != nil {
		return nil, err
	}

	items := make([]PreviewItem, 0, len(skills)*len(selectedAgents))
	for i := range skills {
		for j := range selectedAgents {
			canonicalPath, targetPath, err := resolveInstallPaths(skills[i], selectedAgents[j], env, global)
			if err != nil {
				return nil, err
			}

			willOverwrite := pathExists(targetPath)
			if mode == InstallModeSymlink && pathExists(canonicalPath) {
				willOverwrite = true
			}

			items = append(items, PreviewItem{
				Skill:         skills[i],
				Agent:         selectedAgents[j],
				CanonicalPath: canonicalPath,
				TargetPath:    targetPath,
				WillOverwrite: willOverwrite,
			})
		}
	}

	return items, nil
}

// InstallSelectedSkills installs an explicit skill selection for the requested agents.
func InstallSelectedSkills(
	options ResolverOptions,
	skills []Skill,
	agentNames []string,
	global bool,
	mode InstallMode,
) ([]SuccessItem, []FailureItem, error) {
	if mode == "" {
		mode = InstallModeCopy
	}

	previews, err := PreviewSelectedSkills(options, skills, agentNames, global, mode)
	if err != nil {
		return nil, nil, err
	}

	successes := make([]SuccessItem, 0, len(previews))
	failures := make([]FailureItem, 0)
	for i := range previews {
		sourceFS, err := resolveSkillSource(previews[i].Skill)
		if err != nil {
			failures = append(failures, FailureItem{
				Skill: previews[i].Skill,
				Agent: previews[i].Agent,
				Path:  previews[i].TargetPath,
				Mode:  mode,
				Error: err.Error(),
			})
			continue
		}

		success, failure := installPreviewItem(sourceFS, &previews[i], mode)
		if failure != nil {
			failures = append(failures, *failure)
			continue
		}
		successes = append(successes, *success)
	}

	return successes, failures, nil
}

func resolveSkillSource(skill Skill) (fs.FS, error) {
	if skill.SourceFS != nil && strings.TrimSpace(skill.SourceDir) != "" {
		return skill.SourceFS, nil
	}
	if skill.Origin == AssetOriginBundled && strings.TrimSpace(skill.Directory) != "" {
		return bundledskills.FS, nil
	}
	if strings.TrimSpace(skill.ResolvedPath) != "" {
		resolvedPath := filepath.Clean(strings.TrimSpace(skill.ResolvedPath))
		info, err := os.Stat(resolvedPath)
		if err != nil {
			return nil, fmt.Errorf("stat skill source %q: %w", resolvedPath, err)
		}
		if !info.IsDir() {
			return nil, fmt.Errorf("skill source %q is not a directory", resolvedPath)
		}
		return os.DirFS(filepath.Dir(resolvedPath)), nil
	}
	return nil, fmt.Errorf("skill %q does not declare a source directory", skill.Name)
}
