package setup

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var invalidNamePattern = regexp.MustCompile(`[^a-z0-9._]+`)

var errUnsupportedScope = errors.New("agent does not support this scope")

// SelectSkills filters a discovered catalog to the requested bundled skill names.
func SelectSkills(all []Skill, names []string) ([]Skill, error) {
	return selectByName(all, names, selectByNameConfig[Skill]{
		subject:      "bundled skills",
		emptyLabel:   "skills",
		invalidLabel: "skill(s)",
		getName: func(skill Skill) string {
			return skill.Name
		},
		normalize: strings.TrimSpace,
		less: func(left, right Skill) int {
			return strings.Compare(left.Name, right.Name)
		},
	})
}

// Preview resolves selected bundled skills and agents to on-disk install targets.
func Preview(cfg InstallConfig) ([]PreviewItem, error) {
	if cfg.Bundle == nil {
		return nil, fmt.Errorf("preview bundled setup: bundle is nil")
	}

	allSkills, err := ListSkills(cfg.Bundle)
	if err != nil {
		return nil, err
	}
	selectedSkills, err := SelectSkills(allSkills, cfg.SkillNames)
	if err != nil {
		return nil, err
	}

	allAgents, err := SupportedAgents(cfg.ResolverOptions)
	if err != nil {
		return nil, err
	}
	selectedAgents, err := SelectAgents(allAgents, cfg.AgentNames)
	if err != nil {
		return nil, err
	}

	env, err := resolveEnvironment(cfg.ResolverOptions)
	if err != nil {
		return nil, err
	}

	items := make([]PreviewItem, 0, len(selectedSkills)*len(selectedAgents))
	for i := range selectedSkills {
		for j := range selectedAgents {
			canonicalPath, targetPath, err := resolveInstallPaths(
				selectedSkills[i],
				selectedAgents[j],
				env,
				cfg.Global,
			)
			if err != nil {
				return nil, err
			}

			willOverwrite := pathExists(targetPath)
			if cfg.Mode == InstallModeSymlink && pathExists(canonicalPath) {
				willOverwrite = true
			}

			items = append(items, PreviewItem{
				Skill:         selectedSkills[i],
				Agent:         selectedAgents[j],
				CanonicalPath: canonicalPath,
				TargetPath:    targetPath,
				WillOverwrite: willOverwrite,
			})
		}
	}

	return items, nil
}

// Install materializes bundled skills for the selected agents.
func Install(cfg InstallConfig) (*Result, error) {
	if cfg.Bundle == nil {
		return nil, fmt.Errorf("install bundled skills: bundle is nil")
	}

	mode := cfg.Mode
	if mode == "" {
		mode = InstallModeCopy
	}

	previews, err := Preview(cfg)
	if err != nil {
		return nil, err
	}

	result := &Result{
		Global: cfg.Global,
		Mode:   mode,
	}
	for i := range previews {
		success, failure := installPreviewItem(cfg.Bundle, &previews[i], mode)
		if failure != nil {
			result.Failed = append(result.Failed, *failure)
			continue
		}
		result.Successful = append(result.Successful, *success)
	}
	return result, nil
}

func installPreviewItem(bundle fs.FS, item *PreviewItem, mode InstallMode) (*SuccessItem, *FailureItem) {
	switch mode {
	case InstallModeCopy:
		if err := cleanAndCreateDirectory(item.TargetPath); err != nil {
			return nil, newFailure(item, mode, item.TargetPath, err)
		}
		if err := copyBundleDirectory(bundle, item.Skill.Directory, item.TargetPath, "bundled skill"); err != nil {
			return nil, newFailure(item, mode, item.TargetPath, err)
		}
		return &SuccessItem{
			Skill: item.Skill,
			Agent: item.Agent,
			Path:  item.TargetPath,
			Mode:  mode,
		}, nil
	case InstallModeSymlink:
		if err := cleanAndCreateDirectory(item.CanonicalPath); err != nil {
			return nil, newFailure(item, mode, item.CanonicalPath, err)
		}
		if err := copyBundleDirectory(bundle, item.Skill.Directory, item.CanonicalPath, "bundled skill"); err != nil {
			return nil, newFailure(item, mode, item.CanonicalPath, err)
		}

		symlinkCreated, err := createSymlink(item.CanonicalPath, item.TargetPath)
		if err != nil {
			return nil, newFailure(item, mode, item.TargetPath, err)
		}
		if !symlinkCreated {
			if err := cleanAndCreateDirectory(item.TargetPath); err != nil {
				return nil, newFailure(item, mode, item.TargetPath, err)
			}
			if err := copyBundleDirectory(bundle, item.Skill.Directory, item.TargetPath, "bundled skill"); err != nil {
				return nil, newFailure(item, mode, item.TargetPath, err)
			}
			return &SuccessItem{
				Skill:         item.Skill,
				Agent:         item.Agent,
				Path:          item.TargetPath,
				CanonicalPath: item.CanonicalPath,
				Mode:          mode,
				SymlinkFailed: true,
			}, nil
		}

		path := item.TargetPath
		if samePath(item.CanonicalPath, item.TargetPath) {
			path = item.CanonicalPath
		}
		return &SuccessItem{
			Skill:         item.Skill,
			Agent:         item.Agent,
			Path:          path,
			CanonicalPath: item.CanonicalPath,
			Mode:          mode,
		}, nil
	default:
		return nil, newFailure(item, mode, item.TargetPath, fmt.Errorf("unknown install mode %q", mode))
	}
}

func newFailure(item *PreviewItem, mode InstallMode, path string, err error) *FailureItem {
	return &FailureItem{
		Skill: item.Skill,
		Agent: item.Agent,
		Path:  path,
		Mode:  mode,
		Error: err.Error(),
	}
}

func resolveInstallPaths(
	skill Skill,
	agent Agent,
	env resolvedEnvironment,
	global bool,
) (string, string, error) {
	skillName := sanitizeName(skill.Name)
	canonicalRoot := canonicalSkillsRoot(env, global)
	canonicalPath := filepath.Join(canonicalRoot, skillName)

	agentRoot := agent.ProjectRootDir
	if global {
		agentRoot = agent.GlobalRootDir
	}
	if agent.Universal {
		agentRoot = canonicalRoot
	}
	if agentRoot == "" {
		return "", "", fmt.Errorf(
			"resolve install paths for %q: agent %q does not support this scope: %w",
			skill.Name,
			agent.Name,
			errUnsupportedScope,
		)
	}

	baseDir := env.cwd
	if global {
		baseDir = env.homeDir
	}
	targetRoot := agentRoot
	if !filepath.IsAbs(targetRoot) {
		targetRoot = filepath.Join(baseDir, targetRoot)
	}
	targetPath := filepath.Join(targetRoot, skillName)

	if !isPathSafe(canonicalRoot, canonicalPath) {
		return "", "", fmt.Errorf("resolve install paths for %q: canonical path escaped base directory", skill.Name)
	}
	if !isPathSafe(targetRoot, targetPath) {
		return "", "", fmt.Errorf("resolve install paths for %q: target path escaped base directory", skill.Name)
	}
	return canonicalPath, targetPath, nil
}

func canonicalSkillsRoot(env resolvedEnvironment, global bool) string {
	if global {
		return filepath.Join(env.homeDir, ".agents", "skills")
	}
	return filepath.Join(env.cwd, ".agents", "skills")
}

func sanitizeName(name string) string {
	sanitized := strings.ToLower(strings.TrimSpace(name))
	sanitized = invalidNamePattern.ReplaceAllString(sanitized, "-")
	sanitized = strings.Trim(sanitized, ".-")
	if len(sanitized) > 255 {
		sanitized = sanitized[:255]
	}
	if sanitized == "" {
		return "unnamed-skill"
	}
	return sanitized
}

func isPathSafe(basePath, targetPath string) bool {
	normalizedBase := filepath.Clean(basePath)
	normalizedTarget := filepath.Clean(targetPath)
	return normalizedTarget == normalizedBase ||
		strings.HasPrefix(normalizedTarget, normalizedBase+string(os.PathSeparator))
}

func cleanAndCreateDirectory(path string) error {
	if err := os.RemoveAll(path); err != nil {
		return fmt.Errorf("recreate directory %s: %w", path, err)
	}
	if err := os.MkdirAll(path, 0o755); err != nil {
		return fmt.Errorf("recreate directory %s: %w", path, err)
	}
	return nil
}

func copyBundleDirectory(bundle fs.FS, rootDir, dest string, subject string) error {
	return fs.WalkDir(bundle, rootDir, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relative := strings.TrimPrefix(current, rootDir)
		relative = strings.TrimPrefix(relative, "/")

		target := dest
		if relative != "" {
			target = filepath.Join(dest, filepath.FromSlash(relative))
		}

		if entry.IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return fmt.Errorf("copy %s %s: %w", subject, rootDir, err)
			}
			return nil
		}

		source, err := bundle.Open(current)
		if err != nil {
			return fmt.Errorf("copy %s %s: %w", subject, rootDir, err)
		}
		defer source.Close()

		file, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return fmt.Errorf("copy %s %s: %w", subject, rootDir, err)
		}
		defer file.Close()

		if _, err := io.Copy(file, source); err != nil {
			return fmt.Errorf("copy %s %s: %w", subject, rootDir, err)
		}
		return nil
	})
}

func createSymlink(target, linkPath string) (bool, error) {
	resolvedTarget := filepath.Clean(target)
	resolvedLinkPath := filepath.Clean(linkPath)

	if samePath(resolvedTarget, resolvedLinkPath) {
		return true, nil
	}

	realTarget := evalSymlinksOrSelf(resolvedTarget)
	realLink := evalSymlinksOrSelf(resolvedLinkPath)
	if samePath(realTarget, realLink) {
		return true, nil
	}

	parentResolvedTarget := resolveParentSymlinks(resolvedTarget)
	parentResolvedLink := resolveParentSymlinks(resolvedLinkPath)
	if samePath(parentResolvedTarget, parentResolvedLink) {
		return true, nil
	}

	if info, err := os.Lstat(resolvedLinkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			existingTarget, readErr := os.Readlink(resolvedLinkPath)
			if readErr == nil {
				existingAbsolute := filepath.Clean(filepath.Join(filepath.Dir(resolvedLinkPath), existingTarget))
				if samePath(existingAbsolute, resolvedTarget) {
					return true, nil
				}
			}
		}
		if err := os.RemoveAll(resolvedLinkPath); err != nil {
			return false, nil
		}
	}

	if err := os.MkdirAll(filepath.Dir(resolvedLinkPath), 0o755); err != nil {
		return false, fmt.Errorf("create symlink parent for %s: %w", resolvedLinkPath, err)
	}

	realLinkDir := resolveParentSymlinks(filepath.Dir(resolvedLinkPath))
	relativeTarget, err := filepath.Rel(realLinkDir, resolvedTarget)
	if err != nil {
		return false, fmt.Errorf("create relative symlink for %s: %w", resolvedLinkPath, err)
	}
	if err := os.Symlink(relativeTarget, resolvedLinkPath); err != nil {
		return false, nil
	}
	return true, nil
}

func evalSymlinksOrSelf(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

func resolveParentSymlinks(path string) string {
	resolved := filepath.Clean(path)
	parent := filepath.Dir(resolved)
	base := filepath.Base(resolved)

	realParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return resolved
	}
	return filepath.Join(realParent, base)
}

func samePath(left, right string) bool {
	return filepath.Clean(left) == filepath.Clean(right)
}
