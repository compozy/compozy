package setup

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// Verify checks whether the selected bundled skills are installed and current for one agent.
func Verify(cfg VerifyConfig) (VerifyResult, error) {
	inputs, err := loadVerificationInputs(cfg)
	if err != nil {
		return VerifyResult{}, err
	}

	scope, entries := selectVerificationEntries(inputs.projectEntries, inputs.globalEntries)
	skills, err := verifyEntries(cfg.Bundle, scope, entries)
	if err != nil {
		return VerifyResult{}, err
	}

	return VerifyResult{
		Agent:  inputs.agent,
		Scope:  scope,
		Mode:   detectInstallMode(entries),
		Skills: skills,
	}, nil
}

type verificationInputs struct {
	agent          Agent
	projectEntries []verificationEntry
	globalEntries  []verificationEntry
}

type verificationEntry struct {
	Skill         Skill
	CanonicalPath string
	TargetPath    string
}

func loadVerificationInputs(cfg VerifyConfig) (verificationInputs, error) {
	if cfg.Bundle == nil {
		return verificationInputs{}, fmt.Errorf("verify bundled skills: bundle is nil")
	}

	allSkills, err := ListSkills(cfg.Bundle)
	if err != nil {
		return verificationInputs{}, err
	}

	skillNames := append([]string(nil), cfg.SkillNames...)
	if len(skillNames) == 0 {
		skillNames = bundledSkillNames(allSkills)
	}
	selectedSkills, err := SelectSkills(allSkills, skillNames)
	if err != nil {
		return verificationInputs{}, err
	}

	allAgents, err := SupportedAgents(cfg.ResolverOptions)
	if err != nil {
		return verificationInputs{}, err
	}
	selectedAgents, err := SelectAgents(allAgents, []string{cfg.AgentName})
	if err != nil {
		return verificationInputs{}, err
	}

	env, err := resolveEnvironment(cfg.ResolverOptions)
	if err != nil {
		return verificationInputs{}, err
	}

	agent := selectedAgents[0]
	projectEntries, err := verificationEntries(selectedSkills, agent, env, false)
	if err != nil {
		return verificationInputs{}, err
	}
	globalEntries, err := verificationEntries(selectedSkills, agent, env, true)
	if err != nil {
		return verificationInputs{}, err
	}

	return verificationInputs{
		agent:          agent,
		projectEntries: projectEntries,
		globalEntries:  globalEntries,
	}, nil
}

func selectVerificationEntries(
	projectEntries []verificationEntry,
	globalEntries []verificationEntry,
) (InstallScope, []verificationEntry) {
	switch {
	case hasAnyInstalledSkill(projectEntries):
		return InstallScopeProject, projectEntries
	case hasAnyInstalledSkill(globalEntries):
		return InstallScopeGlobal, globalEntries
	default:
		return InstallScopeUnknown, projectEntries
	}
}

func verifyEntries(bundle fs.FS, scope InstallScope, entries []verificationEntry) ([]VerifiedSkill, error) {
	skills := make([]VerifiedSkill, 0, len(entries))
	for i := range entries {
		verified, err := verifyEntry(bundle, scope, entries[i])
		if err != nil {
			return nil, err
		}
		skills = append(skills, verified)
	}
	return skills, nil
}

func verifyEntry(bundle fs.FS, scope InstallScope, entry verificationEntry) (VerifiedSkill, error) {
	verified := VerifiedSkill{
		Skill:         entry.Skill,
		CanonicalPath: entry.CanonicalPath,
		TargetPath:    entry.TargetPath,
	}

	if scope == InstallScopeUnknown || !pathExists(entry.TargetPath) {
		verified.State = VerifyStateMissing
		return verified, nil
	}

	resolvedPath := resolveInstalledPath(entry.TargetPath)
	verified.ResolvedPath = resolvedPath

	drift, drifted, err := compareInstalledDirectory(bundle, entry.Skill.SourceDir, resolvedPath, "skill")
	if err != nil {
		return VerifiedSkill{}, fmt.Errorf("verify bundled skill %q: %w", entry.Skill.Name, err)
	}
	if drifted {
		verified.State = VerifyStateDrifted
		verified.Drift = drift
		return verified, nil
	}

	verified.State = VerifyStateCurrent
	return verified, nil
}

func verificationEntries(
	skills []Skill,
	agent Agent,
	env resolvedEnvironment,
	global bool,
) ([]verificationEntry, error) {
	items := make([]verificationEntry, 0, len(skills))
	for i := range skills {
		canonicalPath, targetPath, err := resolveInstallPaths(skills[i], agent, env, global)
		if err != nil {
			return nil, err
		}
		items = append(items, verificationEntry{
			Skill:         skills[i],
			CanonicalPath: canonicalPath,
			TargetPath:    targetPath,
		})
	}
	return items, nil
}

func hasAnyInstalledSkill(entries []verificationEntry) bool {
	for i := range entries {
		if pathExists(entries[i].TargetPath) {
			return true
		}
	}
	return false
}

func detectInstallMode(entries []verificationEntry) InstallMode {
	sawSymlink := false
	for i := range entries {
		entry := &entries[i]
		if !pathExists(entry.TargetPath) {
			continue
		}
		if samePath(entry.CanonicalPath, entry.TargetPath) {
			sawSymlink = true
			continue
		}

		info, err := os.Lstat(entry.TargetPath)
		if err != nil {
			continue
		}
		if info.Mode()&os.ModeSymlink != 0 {
			sawSymlink = true
			continue
		}
		return InstallModeCopy
	}

	if sawSymlink {
		return InstallModeSymlink
	}
	return InstallModeCopy
}

func resolveInstalledPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

func compareInstalledDirectory(
	bundle fs.FS,
	sourceDir, installedRoot string,
	subject string,
) (SkillDrift, bool, error) {
	expectedFiles, err := snapshotBundleFiles(bundle, sourceDir, subject)
	if err != nil {
		return SkillDrift{}, false, err
	}

	actualFiles, err := snapshotInstalledFiles(installedRoot, subject)
	if err != nil {
		return SkillDrift{Reason: err.Error()}, true, nil
	}

	drift := SkillDrift{}
	for relativePath, expected := range expectedFiles {
		actual, ok := actualFiles[relativePath]
		if !ok {
			drift.MissingFiles = append(drift.MissingFiles, relativePath)
			continue
		}
		if !bytes.Equal(actual, expected) {
			drift.ChangedFiles = append(drift.ChangedFiles, relativePath)
		}
	}

	for relativePath := range actualFiles {
		if _, ok := expectedFiles[relativePath]; ok {
			continue
		}
		drift.ExtraFiles = append(drift.ExtraFiles, relativePath)
	}

	slices.Sort(drift.MissingFiles)
	slices.Sort(drift.ExtraFiles)
	slices.Sort(drift.ChangedFiles)

	return drift, drift.Reason != "" ||
		len(drift.MissingFiles) > 0 ||
		len(drift.ExtraFiles) > 0 ||
		len(drift.ChangedFiles) > 0, nil
}

func snapshotBundleFiles(bundle fs.FS, root string, subject string) (map[string][]byte, error) {
	files := make(map[string][]byte)
	err := fs.WalkDir(bundle, root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relative := strings.TrimPrefix(current, root)
		relative = strings.TrimPrefix(relative, "/")

		content, err := fs.ReadFile(bundle, current)
		if err != nil {
			return fmt.Errorf("read %s file %q: %w", subject, current, err)
		}
		files[relative] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func snapshotInstalledFiles(root string, subject string) (map[string][]byte, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat installed %s %q: %w", subject, root, err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("installed %s path %q is not a directory", subject, root)
	}

	rootFS, err := os.OpenRoot(root)
	if err != nil {
		return nil, fmt.Errorf("open installed %s root %q: %w", subject, root, err)
	}
	defer rootFS.Close()

	files := make(map[string][]byte)
	err = filepath.WalkDir(root, func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}

		relative, err := filepath.Rel(root, current)
		if err != nil {
			return fmt.Errorf("resolve relative path for %q: %w", current, err)
		}

		content, err := rootFS.ReadFile(filepath.ToSlash(relative))
		if err != nil {
			return fmt.Errorf("read installed %s file %q: %w", subject, current, err)
		}
		files[filepath.ToSlash(relative)] = content
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

func bundledSkillNames(skills []Skill) []string {
	names := make([]string, 0, len(skills))
	for i := range skills {
		names = append(names, skills[i].Name)
	}
	return names
}
