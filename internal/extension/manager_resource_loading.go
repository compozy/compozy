package extensionpkg

import (
	"context"

	"errors"
	"fmt"

	"path/filepath"

	"strings"

	"time"

	aghconfig "github.com/compozy/agh/internal/config"

	hookspkg "github.com/compozy/agh/internal/hooks"
	looppkg "github.com/compozy/agh/internal/loop"

	skillspkg "github.com/compozy/agh/internal/skills"
)

func (m *Manager) loadSkillResources(ext *managedExtension) ([]*skillspkg.Skill, error) {
	if ext.manifest == nil || len(ext.manifest.Resources.Skills) == 0 {
		return nil, nil
	}

	source := skillSourceForExtension(ext.info.Source)
	loaded := make(map[string]*skillspkg.Skill)
	for _, resourcePath := range ext.manifest.Resources.Skills {
		resourceRoot, err := resolveResourcePath(ext.rootDir, resourcePath)
		if err != nil {
			return nil, err
		}
		files, err := collectMarkdownFiles(resourceRoot)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			skill, err := skillspkg.ParseSkillFileWithSource(file, source)
			if err != nil {
				return nil, err
			}
			skill.InstalledFromExtension = extensionSkillInstalledFrom(ext.info)
			loaded[skill.Meta.Name] = skill
		}
	}

	skills := make([]*skillspkg.Skill, 0, len(loaded))
	for _, name := range sortedKeys(loaded) {
		skills = append(skills, loaded[name])
	}
	return skills, nil
}

func extensionSkillInstalledFrom(info ExtensionInfo) string {
	if info.RegistrySlug != nil {
		if slug := strings.TrimSpace(*info.RegistrySlug); slug != "" {
			return slug
		}
	}
	if slug := strings.TrimSpace(info.Provenance.Slug); slug != "" {
		return slug
	}
	return strings.TrimSpace(info.Name)
}

func (m *Manager) loadLoopResources(ext *managedExtension) ([]looppkg.ResourceSpec, error) {
	if ext.manifest == nil || len(ext.manifest.Resources.Loops) == 0 {
		return nil, nil
	}

	source := loopSourceForExtension(ext.info.Source)
	loaded := make(map[string]looppkg.ResourceSpec)
	for _, resourcePath := range ext.manifest.Resources.Loops {
		resourceRoot, err := resolveResourcePath(ext.rootDir, resourcePath)
		if err != nil {
			return nil, err
		}
		files, err := collectLoopDefinitionFiles(resourceRoot)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			spec, _, err := looppkg.ParseResourceFile(file, looppkg.ResourceParseOptions{
				Source:                 source,
				InstalledFromExtension: extensionSkillInstalledFrom(ext.info),
			})
			if err != nil {
				return nil, err
			}
			if dirName := filepath.Base(filepath.Dir(file)); dirName != spec.Name {
				return nil, fmt.Errorf(
					"loop resource %q directory name %q does not match loop name %q",
					file,
					dirName,
					spec.Name,
				)
			}
			if previous, exists := loaded[spec.Name]; exists {
				return nil, fmt.Errorf(
					"duplicate loop resource %q in extension %q: %s and %s",
					spec.Name,
					ext.info.Name,
					previous.FilePath,
					spec.FilePath,
				)
			}
			loaded[spec.Name] = spec
		}
	}

	loops := make([]looppkg.ResourceSpec, 0, len(loaded))
	for _, name := range sortedKeys(loaded) {
		loops = append(loops, looppkg.CloneResourceSpec(loaded[name]))
	}
	return loops, nil
}

func (m *Manager) loadAgentResources(ext *managedExtension) ([]aghconfig.AgentDef, error) {
	if ext == nil {
		return nil, nil
	}
	return LoadAgentResources(ext.rootDir, ext.manifest)
}

// LoadAgentResources discovers the current authored agent definitions for an extension.
func LoadAgentResources(rootDir string, manifest *Manifest) ([]aghconfig.AgentDef, error) {
	if manifest == nil || len(manifest.Resources.Agents) == 0 {
		return nil, nil
	}

	loaded := make(map[string]aghconfig.AgentDef)
	for _, resourcePath := range manifest.Resources.Agents {
		resourceRoot, err := resolveResourcePath(rootDir, resourcePath)
		if err != nil {
			return nil, err
		}
		files, err := collectMarkdownFiles(resourceRoot)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			agent, err := aghconfig.LoadAgentDefFile(file)
			if err != nil {
				return nil, err
			}
			loaded[agent.Name] = agent
		}
	}

	agents := make([]aghconfig.AgentDef, 0, len(loaded))
	for _, name := range sortedKeys(loaded) {
		agents = append(agents, aghconfig.CloneAgentDef(loaded[name]))
	}
	return agents, nil
}

func (m *Manager) loadHookResources(ext *managedExtension) ([]hookspkg.HookDecl, error) {
	if ext.manifest == nil || len(ext.manifest.Resources.Hooks) == 0 {
		return nil, nil
	}

	decls := make([]hookspkg.HookDecl, 0, len(ext.manifest.Resources.Hooks))
	for idx := range ext.manifest.Resources.Hooks {
		cfg := &ext.manifest.Resources.Hooks[idx]
		decl, err := m.hookConfigToDecl(ext, cfg)
		if err != nil {
			return nil, fmt.Errorf("extension hook %d (%q): %w", idx, strings.TrimSpace(cfg.Name), err)
		}
		decls = append(decls, decl)
	}
	return decls, nil
}

func (m *Manager) loadBundleResources(ctx context.Context, ext *managedExtension) ([]BundleSpec, error) {
	if ext == nil || ext.manifest == nil {
		return nil, nil
	}
	return LoadBundleSpecs(ctx, ext.rootDir, ext.manifest)
}

func (m *Manager) hookConfigToDecl(ext *managedExtension, cfg *HookConfig) (hookspkg.HookDecl, error) {
	if cfg == nil {
		return hookspkg.HookDecl{}, errors.New("hook config is required")
	}
	executor, err := resolveHookConfigExecutorFields(cfg)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}
	resolvedCommand, err := m.resolveCommand(ext.rootDir, executor.command)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}
	resolvedArgs, err := m.resolveStringSlice(ext.rootDir, executor.args)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}
	resolvedEnv, err := m.resolveStringMap(ext.rootDir, executor.env)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}

	decl := hookspkg.HookDecl{
		Name:         strings.TrimSpace(cfg.Name),
		Event:        hookspkg.HookEvent(strings.TrimSpace(cfg.Event)),
		Source:       extensionHookSource,
		Mode:         hookspkg.HookMode(strings.TrimSpace(cfg.Mode)),
		Required:     cfg.Required,
		Timeout:      time.Duration(cfg.Timeout),
		Matcher:      hookConfigMatcher(cfg.Matcher),
		ExecutorKind: executor.kind,
		Command:      resolvedCommand,
		Args:         resolvedArgs,
		WorkingDir:   ext.rootDir,
		Env:          resolvedEnv,
		SecretEnv:    executor.secretEnv,
		Metadata: map[string]string{
			managerExtensionKey: ext.info.Name,
		},
	}
	if cfg.Priority != nil {
		priority, err := hookspkg.PriorityFromInt(*cfg.Priority)
		if err != nil {
			return hookspkg.HookDecl{}, err
		}
		decl.Priority = priority
		decl.PrioritySet = true
	}

	if err := hookspkg.ValidateHookDecl(decl); err != nil {
		return hookspkg.HookDecl{}, err
	}
	return decl, nil
}
