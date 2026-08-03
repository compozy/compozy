package extensionpkg

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	automationpkg "github.com/compozy/compozy/internal/automation"
	compozyconfig "github.com/compozy/compozy/internal/config"
	hookspkg "github.com/compozy/compozy/internal/hooks"
	looppkg "github.com/compozy/compozy/internal/loop"
	skillspkg "github.com/compozy/compozy/internal/skills"
	"github.com/compozy/compozy/internal/windowmanager"
)

type loadedExtensionResources struct {
	skills             []*skillspkg.Skill
	loops              []looppkg.ResourceSpec
	staticAgents       []StaticAgent
	agents             []compozyconfig.AgentDef
	automationJobs     []automationpkg.Job
	automationTriggers []automationpkg.Trigger
	layouts            []windowmanager.LayoutResource
	hooks              []hookspkg.HookDecl
}

func (resources loadedExtensionResources) apply(ext *managedExtension) {
	ext.skills = resources.skills
	ext.loops = resources.loops
	ext.staticAgents = resources.staticAgents
	ext.agents = resources.agents
	ext.automationJobs = resources.automationJobs
	ext.automationTriggers = resources.automationTriggers
	ext.layouts = resources.layouts
	ext.hooks = resources.hooks
}

func (m *Manager) loadDeclarativeResources(
	ctx context.Context,
	ext *managedExtension,
) (loadedExtensionResources, error) {
	if ctx == nil {
		return loadedExtensionResources{}, errors.New("extension: declarative resource context is required")
	}
	if err := ctx.Err(); err != nil {
		return loadedExtensionResources{}, err
	}

	var loaded loadedExtensionResources
	var err error
	loaded.staticAgents, err = m.loadAgentResources(ext)
	if err != nil {
		return loadedExtensionResources{}, fmt.Errorf("extension: load declared agents: %w", err)
	}
	loaded.agents = make([]compozyconfig.AgentDef, 0, len(loaded.staticAgents))
	for _, agent := range loaded.staticAgents {
		loaded.agents = append(loaded.agents, compozyconfig.CloneAgentDef(agent.Agent))
	}
	loaded.skills, err = m.loadSkillResources(ext)
	if err != nil {
		return loadedExtensionResources{}, fmt.Errorf("extension: load declared skills: %w", err)
	}
	loaded.loops, err = m.loadLoopResources(ext)
	if err != nil {
		return loadedExtensionResources{}, fmt.Errorf("extension: load declared loops: %w", err)
	}
	loaded.automationJobs, loaded.automationTriggers, err = m.loadAutomationResources(ext, loaded.staticAgents)
	if err != nil {
		return loadedExtensionResources{}, fmt.Errorf("extension: load declared automation: %w", err)
	}
	loaded.layouts, err = m.loadLayoutResources(ctx, ext)
	if err != nil {
		return loadedExtensionResources{}, fmt.Errorf("extension: load declared layouts: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return loadedExtensionResources{}, err
	}
	loaded.hooks, err = m.loadHookResources(ext)
	if err != nil {
		return loadedExtensionResources{}, fmt.Errorf("extension: load declared hooks: %w", err)
	}
	return loaded, nil
}

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
		files, err := collectSkillDefinitionFiles(resourceRoot)
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

func (m *Manager) loadAgentResources(ext *managedExtension) ([]StaticAgent, error) {
	if ext == nil {
		return nil, nil
	}
	if ext.manifest == nil {
		return nil, nil
	}
	return LoadAgentResources(ext.rootDir, ext.manifest.Resources.Agents)
}

func (m *Manager) loadAutomationResources(
	ext *managedExtension,
	agents []StaticAgent,
) ([]automationpkg.Job, []automationpkg.Trigger, error) {
	if ext == nil || ext.manifest == nil {
		return nil, nil, nil
	}
	jobs, triggers, err := LoadAutomationResources(
		ext.rootDir,
		ext.info.Name,
		ext.manifest.Resources.Automation,
	)
	if err != nil {
		return nil, nil, err
	}
	if err := validateAutomationAgentTargets(jobs, triggers, agents); err != nil {
		return nil, nil, err
	}
	return jobs, triggers, nil
}

func (m *Manager) loadLayoutResources(
	ctx context.Context,
	ext *managedExtension,
) ([]windowmanager.LayoutResource, error) {
	if ext == nil || ext.manifest == nil {
		return nil, nil
	}
	return LoadLayoutResources(ctx, ext.rootDir, ext.manifest.Resources.Layouts)
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
		Source:       hookspkg.HookSourceExtension,
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
	} else {
		priority, err := hookspkg.DefaultHookPriority(hookspkg.HookSourceExtension)
		if err != nil {
			return hookspkg.HookDecl{}, fmt.Errorf("resolve extension hook priority: %w", err)
		}
		decl.Priority = priority
	}

	if err := hookspkg.ValidateHookDecl(decl); err != nil {
		return hookspkg.HookDecl{}, err
	}
	return decl, nil
}
