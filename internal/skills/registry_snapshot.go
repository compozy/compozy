package skills

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

func recordSidecarSnapshots(paths []string, snapshots map[string]filesnap.Snapshot) error {
	for _, skillPath := range paths {
		for _, sidecarPath := range []string{
			filepath.Join(filepath.Dir(skillPath), sidecarFileName),
			filepath.Join(filepath.Dir(skillPath), aghconfig.MCPJSONName),
		} {
			snapshot, err := filesnap.FromPath(sidecarPath)
			if err != nil {
				if errors.Is(err, fs.ErrNotExist) {
					continue
				}

				return fmt.Errorf("skills: snapshot sidecar %q: %w", sidecarPath, err)
			}

			snapshots[sidecarPath] = snapshot
		}
	}

	return nil
}

func hasCriticalWarning(warnings []Warning) bool {
	for _, warning := range warnings {
		if warning.Severity == SeverityCritical {
			return true
		}
	}
	return false
}

func mergedSkillList(globalSkills, workspaceSkills map[string]*Skill) []*Skill {
	if len(globalSkills) == 0 && len(workspaceSkills) == 0 {
		return nil
	}
	if len(workspaceSkills) == 0 {
		return cloneSortedSkillList(globalSkills)
	}
	if len(globalSkills) == 0 {
		return cloneSortedSkillList(workspaceSkills)
	}

	names := make([]string, 0, len(globalSkills)+len(workspaceSkills))
	for name := range globalSkills {
		names = append(names, name)
	}
	for name := range workspaceSkills {
		names = append(names, name)
	}
	slices.Sort(names)

	skills := make([]*Skill, 0, len(names))
	previous := ""
	havePrevious := false
	for _, name := range names {
		if havePrevious && name == previous {
			continue
		}
		havePrevious = true
		previous = name

		if skill, ok := workspaceSkills[name]; ok {
			cloned := cloneSkill(skill)
			if global := globalSkills[name]; global != nil {
				cloned.Diagnostics.ShadowedDefinitions = append(
					cloneSkillDefinitionRefs(cloned.Diagnostics.ShadowedDefinitions),
					shadowDefinitionRefsForWinner(global, time.Time{})...,
				)
			}
			skills = append(skills, cloned)
			continue
		}
		skills = append(skills, cloneSkill(globalSkills[name]))
	}

	return skills
}

func mergedSkillListWithDisabled(globalSkills, workspaceSkills map[string]*Skill, disabledSkills []string) []*Skill {
	skills := mergedSkillList(globalSkills, workspaceSkills)
	applyDisabledSkillList(skills, disabledSkills)
	return skills
}

func applyDisabledSkillList(skills []*Skill, disabledSkills []string) {
	if len(skills) == 0 || len(disabledSkills) == 0 {
		return
	}

	disabled := make(map[string]struct{}, len(disabledSkills))
	for _, name := range disabledSkills {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		disabled[trimmed] = struct{}{}
	}
	if len(disabled) == 0 {
		return
	}

	for _, skill := range skills {
		if skill == nil {
			continue
		}
		if _, ok := disabled[strings.TrimSpace(skill.Meta.Name)]; ok {
			skill.Enabled = false
		}
	}
}

func cloneSortedSkillList(skillsByName map[string]*Skill) []*Skill {
	if len(skillsByName) == 0 {
		return nil
	}

	names := make([]string, 0, len(skillsByName))
	for name := range skillsByName {
		names = append(names, name)
	}
	slices.Sort(names)

	cloned := make([]*Skill, 0, len(names))
	for _, name := range names {
		cloned = append(cloned, cloneSkill(skillsByName[name]))
	}
	return cloned
}

func cloneSkill(skill *Skill) *Skill {
	if skill == nil {
		return nil
	}

	clone := *skill
	clone.Meta = cloneSkillMeta(skill.Meta)
	clone.ActivationGates = cloneActivationGates(skill.ActivationGates)
	clone.Activation = cloneSkillActivation(skill.Activation)
	clone.MCPServers = cloneMCPServerDecls(skill.MCPServers)
	if len(skill.Hooks) > 0 {
		clone.Hooks = make([]hookspkg.HookDecl, 0, len(skill.Hooks))
		for idx, decl := range skill.Hooks {
			cloned := decl
			cloned.Args = append([]string(nil), decl.Args...)
			cloned.Env = cloneStringMap(decl.Env)
			cloned.SecretEnv = cloneStringMap(decl.SecretEnv)
			cloned.Metadata = cloneStringMap(decl.Metadata)
			if decl.Matcher.ToolReadOnly != nil {
				value := *decl.Matcher.ToolReadOnly
				cloned.Matcher.ToolReadOnly = &value
			}
			clone.Hooks = append(clone.Hooks, normalizeSkillHookDecl(skill, cloned, idx, len(skill.Hooks)))
		}
	}
	clone.Provenance = cloneProvenance(skill.Provenance)
	clone.Diagnostics = cloneSkillDiagnostics(skill.Diagnostics)

	return &clone
}

func cloneSkillMeta(meta SkillMeta) SkillMeta {
	clone := meta
	clone.Metadata = cloneMetadataMap(meta.Metadata)
	return clone
}

func cloneMetadataMap(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}

	clone := make(map[string]any, len(metadata))
	for key, value := range metadata {
		clone[key] = cloneMetadataValue(value)
	}

	return clone
}

func cloneMetadataValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneMetadataMap(typed)
	case []any:
		clone := make([]any, len(typed))
		for i := range typed {
			clone[i] = cloneMetadataValue(typed[i])
		}
		return clone
	default:
		return typed
	}
}

func cloneMCPServerDecls(decls []MCPServerDecl) []MCPServerDecl {
	if decls == nil {
		return nil
	}

	clone := make([]MCPServerDecl, len(decls))
	for i, decl := range decls {
		clone[i] = MCPServerDecl{
			Name:      decl.Name,
			Command:   decl.Command,
			Args:      append([]string(nil), decl.Args...),
			Env:       cloneStringMap(decl.Env),
			SecretEnv: cloneStringMap(decl.SecretEnv),
		}
	}

	return clone
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}

	clone := make(map[string]string, len(input))
	maps.Copy(clone, input)

	return clone
}

func cloneProvenance(provenance *Provenance) *Provenance {
	if provenance == nil {
		return nil
	}

	clone := *provenance
	return &clone
}

func (r *Registry) globalSnapshotState() (map[string]filesnap.Snapshot, bool) {
	if r == nil {
		return make(map[string]filesnap.Snapshot), false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	return filesnap.Clone(r.globalSnapshots), r.globalLoaded
}

func parseBundledSkill(fsys fs.FS, skillPath string) (*Skill, error) {
	skill, _, err := parseBundledSkillDocument(fsys, skillPath)
	return skill, err
}

func readBundledSkillContent(fsys fs.FS, skillPath string) (string, error) {
	_, body, err := parseBundledSkillDocument(fsys, skillPath)
	if err != nil {
		return "", err
	}
	return body, nil
}

func parseBundledSkillDocument(fsys fs.FS, skillPath string) (*Skill, string, error) {
	content, err := fs.ReadFile(fsys, skillPath)
	if err != nil {
		return nil, "", fmt.Errorf("skills: read bundled skill %q: %w", skillPath, err)
	}

	dir := path.Dir(skillPath)
	if dir == "." {
		dir = ""
	}

	skill, body, err := parseSkillDocument(skillPath, dir, content, SourceBundled)
	if err != nil {
		return nil, "", err
	}
	if err := mergeSkillMCPSidecarFS(fsys, dir, skill); err != nil {
		return nil, "", fmt.Errorf("skills: parse bundled skill %q MCP JSON: %w", skillPath, err)
	}

	return skill, body, nil
}

func scanBundledFS(fsys fs.FS) ([]string, error) {
	paths := make([]string, 0, maxScanCandidates)

	walkErr := fs.WalkDir(fsys, ".", func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == "." {
			return nil
		}

		depth := fsPathDepth(current, entry.IsDir())
		if entry.IsDir() {
			if shouldSkipDir(path.Base(current)) {
				return fs.SkipDir
			}
			if depth > maxScanDepth {
				return fs.SkipDir
			}
			return nil
		}

		if path.Base(current) != skillFileName || depth > maxScanDepth {
			return nil
		}

		if _, err := fs.Stat(fsys, current); err != nil {
			return err
		}

		paths = append(paths, current)
		if len(paths) >= maxScanCandidates {
			return errScanLimitReached
		}

		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, errScanLimitReached) {
		return nil, fmt.Errorf("skills: scan bundled skills: %w", walkErr)
	}

	slices.Sort(paths)
	return paths, nil
}

func fsPathDepth(current string, isDir bool) int {
	trimmed := strings.Trim(current, "/")
	if trimmed == "" {
		return 0
	}

	parts := strings.Split(trimmed, "/")
	if isDir {
		return len(parts)
	}

	return max(len(parts)-1, 0)
}
