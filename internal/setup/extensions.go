package setup

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ExtensionInstallConfig describes one extension-skill installation run.
type ExtensionInstallConfig struct {
	ResolverOptions

	Packs      []SkillPackSource
	AgentNames []string
	Global     bool
	Mode       InstallMode
}

// ExtensionVerifyConfig describes one extension-skill verification run.
type ExtensionVerifyConfig struct {
	ResolverOptions

	Packs     []SkillPackSource
	AgentName string
	ScopeHint InstallScope
}

// SkillPackSource captures one declarative skill-pack source resolved during extension discovery.
type SkillPackSource struct {
	ExtensionName   string
	ExtensionSource string
	ManifestPath    string
	Pattern         string
	ResolvedPath    string
	SourceFS        fs.FS
	SourceDir       string
}

// ExtensionPreviewItem describes the on-disk plan for one extension skill/agent install pair.
type ExtensionPreviewItem struct {
	Pack SkillPackSource
	PreviewItem
}

// ExtensionSuccessItem captures one successful extension skill installation mapping.
type ExtensionSuccessItem struct {
	Pack SkillPackSource
	SuccessItem
}

// ExtensionFailureItem captures one failed extension skill installation mapping.
type ExtensionFailureItem struct {
	Pack SkillPackSource
	FailureItem
}

// ExtensionResult summarizes one extension-skill installation run.
type ExtensionResult struct {
	Global     bool
	Mode       InstallMode
	Successful []ExtensionSuccessItem
	Failed     []ExtensionFailureItem
}

// ExtensionVerifiedSkill captures the verification result for one extension skill pack.
type ExtensionVerifiedSkill struct {
	Pack SkillPackSource
	VerifiedSkill
}

// ExtensionVerifyResult summarizes one extension-skill verification run.
type ExtensionVerifyResult struct {
	Agent  Agent
	Scope  InstallScope
	Mode   InstallMode
	Skills []ExtensionVerifiedSkill
}

// MissingSkillNames returns every missing extension skill name in sorted order.
func (r ExtensionVerifyResult) MissingSkillNames() []string {
	names := make([]string, 0, len(r.Skills))
	for i := range r.Skills {
		skill := &r.Skills[i]
		if skill.State != VerifyStateMissing {
			continue
		}
		names = append(names, skill.Skill.Name)
	}
	slices.Sort(names)
	return names
}

// DriftedSkillNames returns every drifted extension skill name in sorted order.
func (r ExtensionVerifyResult) DriftedSkillNames() []string {
	names := make([]string, 0, len(r.Skills))
	for i := range r.Skills {
		skill := &r.Skills[i]
		if skill.State != VerifyStateDrifted {
			continue
		}
		names = append(names, skill.Skill.Name)
	}
	slices.Sort(names)
	return names
}

// HasMissing reports whether any extension skill is missing.
func (r ExtensionVerifyResult) HasMissing() bool {
	for i := range r.Skills {
		if r.Skills[i].State == VerifyStateMissing {
			return true
		}
	}
	return false
}

// HasDrift reports whether any extension skill differs from its declared source.
func (r ExtensionVerifyResult) HasDrift() bool {
	for i := range r.Skills {
		if r.Skills[i].State == VerifyStateDrifted {
			return true
		}
	}
	return false
}

type extensionSkillSource struct {
	Pack   SkillPackSource
	Skill  Skill
	Source fs.FS
}

type extensionVerificationEntry struct {
	Source        extensionSkillSource
	CanonicalPath string
	TargetPath    string
}

type extensionInstallPreview struct {
	Source      extensionSkillSource
	PreviewItem PreviewItem
}

// InstallExtensionSkillPacks materializes enabled extension skill packs for the selected agents.
func InstallExtensionSkillPacks(cfg ExtensionInstallConfig) (*ExtensionResult, error) {
	mode := cfg.Mode
	if mode == "" {
		mode = InstallModeCopy
	}

	sources, err := loadExtensionSkillSources(cfg.Packs)
	if err != nil {
		return nil, err
	}

	selectedAgents, env, err := resolveSelectedAgentsForExtensionSkills(cfg.ResolverOptions, cfg.AgentNames)
	if err != nil {
		return nil, err
	}

	previews, err := previewExtensionSkillInstall(sources, selectedAgents, env, cfg.Global)
	if err != nil {
		return nil, err
	}

	result := &ExtensionResult{
		Global: cfg.Global,
		Mode:   mode,
	}
	for i := range previews {
		preview := &previews[i]
		success, failure := installPreviewItem(preview.Source.Source, &preview.PreviewItem, mode)
		if failure != nil {
			result.Failed = append(result.Failed, ExtensionFailureItem{
				Pack:        preview.Source.Pack,
				FailureItem: *failure,
			})
			continue
		}
		result.Successful = append(result.Successful, ExtensionSuccessItem{
			Pack:        preview.Source.Pack,
			SuccessItem: *success,
		})
	}

	return result, nil
}

// ListExtensionSkills enumerates installable skill assets declared by enabled extensions.
func ListExtensionSkills(packs []SkillPackSource) ([]Skill, error) {
	sources, err := loadExtensionSkillSources(packs)
	if err != nil {
		return nil, err
	}

	skills := make([]Skill, 0, len(sources))
	for i := range sources {
		skills = append(skills, sources[i].Skill)
	}
	return skills, nil
}

// VerifyExtensionSkillPacks checks whether declared extension skill packs are installed and current.
func VerifyExtensionSkillPacks(cfg ExtensionVerifyConfig) (ExtensionVerifyResult, error) {
	sources, err := loadExtensionSkillSources(cfg.Packs)
	if err != nil {
		return ExtensionVerifyResult{}, err
	}

	agent, env, err := resolveSingleAgentForExtensionSkills(cfg.ResolverOptions, cfg.AgentName)
	if err != nil {
		return ExtensionVerifyResult{}, err
	}

	projectEntries, err := extensionVerificationEntries(sources, agent, env, false)
	if err != nil {
		return ExtensionVerifyResult{}, err
	}
	globalEntries, err := extensionVerificationEntries(sources, agent, env, true)
	if err != nil {
		return ExtensionVerifyResult{}, err
	}

	scope, selectedEntries := selectExtensionVerificationEntries(projectEntries, globalEntries, cfg.ScopeHint)
	skills, err := verifyExtensionEntries(scope, selectedEntries)
	if err != nil {
		return ExtensionVerifyResult{}, err
	}

	return ExtensionVerifyResult{
		Agent:  agent,
		Scope:  scope,
		Mode:   detectExtensionInstallMode(selectedEntries),
		Skills: skills,
	}, nil
}

func resolveSelectedAgentsForExtensionSkills(
	options ResolverOptions,
	agentNames []string,
) ([]Agent, resolvedEnvironment, error) {
	allAgents, err := SupportedAgents(options)
	if err != nil {
		return nil, resolvedEnvironment{}, err
	}
	selectedAgents, err := SelectAgents(allAgents, agentNames)
	if err != nil {
		return nil, resolvedEnvironment{}, err
	}
	env, err := resolveEnvironment(options)
	if err != nil {
		return nil, resolvedEnvironment{}, err
	}
	return selectedAgents, env, nil
}

func resolveSingleAgentForExtensionSkills(
	options ResolverOptions,
	agentName string,
) (Agent, resolvedEnvironment, error) {
	selectedAgents, env, err := resolveSelectedAgentsForExtensionSkills(options, []string{agentName})
	if err != nil {
		return Agent{}, resolvedEnvironment{}, err
	}
	return selectedAgents[0], env, nil
}

func loadExtensionSkillSources(packs []SkillPackSource) ([]extensionSkillSource, error) {
	if len(packs) == 0 {
		return nil, nil
	}

	sources := make([]extensionSkillSource, 0, len(packs))
	for _, pack := range packs {
		sourceFS, sourceDir, err := resolveDeclaredSkillPackSource(pack)
		if err != nil {
			return nil, err
		}
		skill, err := parseSkill(sourceFS, sourceDir)
		if err != nil {
			return nil, fmt.Errorf(
				"load extension skill pack %q from %q: %w",
				pack.ExtensionName,
				pack.ResolvedPath,
				err,
			)
		}
		sources = append(sources, extensionSkillSource{
			Pack: pack,
			Skill: Skill{
				Name:            skill.Name,
				Description:     skill.Description,
				Directory:       skill.Directory,
				Origin:          AssetOriginExtension,
				ExtensionName:   pack.ExtensionName,
				ExtensionSource: pack.ExtensionSource,
				ManifestPath:    pack.ManifestPath,
				ResolvedPath:    pack.ResolvedPath,
				SourceFS:        sourceFS,
				SourceDir:       sourceDir,
			},
			Source: sourceFS,
		})
	}

	slices.SortFunc(sources, compareExtensionSkillSource)
	return sources, nil
}

func resolveDeclaredSkillPackSource(pack SkillPackSource) (fs.FS, string, error) {
	if pack.SourceFS != nil && strings.TrimSpace(pack.SourceDir) != "" {
		return pack.SourceFS, strings.TrimSpace(pack.SourceDir), nil
	}

	resolvedPath := filepath.Clean(strings.TrimSpace(pack.ResolvedPath))
	if resolvedPath == "" {
		return nil, "", fmt.Errorf("extension skill pack source path is required")
	}
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, "", fmt.Errorf("stat extension skill pack %q: %w", resolvedPath, err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("extension skill pack %q is not a directory", resolvedPath)
	}

	parentDir := filepath.Dir(resolvedPath)
	sourceDir := filepath.Base(resolvedPath)
	return os.DirFS(parentDir), sourceDir, nil
}

func compareExtensionSkillSource(left, right extensionSkillSource) int {
	if diff := strings.Compare(left.Skill.Name, right.Skill.Name); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.Pack.ExtensionName, right.Pack.ExtensionName); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.Pack.ManifestPath, right.Pack.ManifestPath); diff != 0 {
		return diff
	}
	return strings.Compare(left.Pack.ResolvedPath, right.Pack.ResolvedPath)
}

func previewExtensionSkillInstall(
	sources []extensionSkillSource,
	agents []Agent,
	env resolvedEnvironment,
	global bool,
) ([]extensionInstallPreview, error) {
	items := make([]extensionInstallPreview, 0, len(sources)*len(agents))
	for i := range sources {
		source := sources[i]
		for _, agent := range agents {
			canonicalPath, targetPath, err := resolveInstallPaths(source.Skill, agent, env, global)
			if err != nil {
				return nil, err
			}

			willOverwrite := pathExists(targetPath)
			items = append(items, extensionInstallPreview{
				Source: source,
				PreviewItem: PreviewItem{
					Skill:         source.Skill,
					Agent:         agent,
					CanonicalPath: canonicalPath,
					TargetPath:    targetPath,
					WillOverwrite: willOverwrite,
				},
			})
		}
	}
	return items, nil
}

func extensionVerificationEntries(
	sources []extensionSkillSource,
	agent Agent,
	env resolvedEnvironment,
	global bool,
) ([]extensionVerificationEntry, error) {
	items := make([]extensionVerificationEntry, 0, len(sources))
	for i := range sources {
		source := sources[i]
		canonicalPath, targetPath, err := resolveInstallPaths(source.Skill, agent, env, global)
		if err != nil {
			return nil, err
		}
		items = append(items, extensionVerificationEntry{
			Source:        source,
			CanonicalPath: canonicalPath,
			TargetPath:    targetPath,
		})
	}
	return items, nil
}

func selectExtensionVerificationEntries(
	projectEntries []extensionVerificationEntry,
	globalEntries []extensionVerificationEntry,
	scopeHint InstallScope,
) (InstallScope, []extensionVerificationEntry) {
	switch scopeHint {
	case InstallScopeProject:
		return InstallScopeProject, projectEntries
	case InstallScopeGlobal:
		return InstallScopeGlobal, globalEntries
	}

	switch {
	case hasAnyInstalledExtensionSkill(projectEntries):
		return InstallScopeProject, projectEntries
	case hasAnyInstalledExtensionSkill(globalEntries):
		return InstallScopeGlobal, globalEntries
	default:
		return InstallScopeUnknown, projectEntries
	}
}

func hasAnyInstalledExtensionSkill(entries []extensionVerificationEntry) bool {
	for i := range entries {
		entry := entries[i]
		if pathExists(entry.TargetPath) {
			return true
		}
	}
	return false
}

func verifyExtensionEntries(
	scope InstallScope,
	entries []extensionVerificationEntry,
) ([]ExtensionVerifiedSkill, error) {
	skills := make([]ExtensionVerifiedSkill, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		verified, err := verifyExtensionEntry(scope, entry)
		if err != nil {
			return nil, err
		}
		skills = append(skills, verified)
	}
	return skills, nil
}

func verifyExtensionEntry(scope InstallScope, entry extensionVerificationEntry) (ExtensionVerifiedSkill, error) {
	verified := VerifiedSkill{
		Skill:         entry.Source.Skill,
		CanonicalPath: entry.CanonicalPath,
		TargetPath:    entry.TargetPath,
	}

	if scope == InstallScopeUnknown || !pathExists(entry.TargetPath) {
		verified.State = VerifyStateMissing
		return ExtensionVerifiedSkill{
			Pack:          entry.Source.Pack,
			VerifiedSkill: verified,
		}, nil
	}

	resolvedPath := resolveInstalledPath(entry.TargetPath)
	verified.ResolvedPath = resolvedPath

	drift, drifted, err := compareInstalledDirectory(
		entry.Source.Source,
		entry.Source.Skill.SourceDir,
		resolvedPath,
		"skill",
	)
	if err != nil {
		return ExtensionVerifiedSkill{}, fmt.Errorf(
			"verify extension skill %q from %q: %w",
			entry.Source.Skill.Name,
			entry.Source.Pack.ResolvedPath,
			err,
		)
	}
	if drifted {
		verified.State = VerifyStateDrifted
		verified.Drift = drift
		return ExtensionVerifiedSkill{
			Pack:          entry.Source.Pack,
			VerifiedSkill: verified,
		}, nil
	}

	verified.State = VerifyStateCurrent
	return ExtensionVerifiedSkill{
		Pack:          entry.Source.Pack,
		VerifiedSkill: verified,
	}, nil
}

func detectExtensionInstallMode(entries []extensionVerificationEntry) InstallMode {
	baseEntries := make([]verificationEntry, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		baseEntries = append(baseEntries, verificationEntry{
			Skill:         entry.Source.Skill,
			CanonicalPath: entry.CanonicalPath,
			TargetPath:    entry.TargetPath,
		})
	}
	return detectInstallMode(baseEntries)
}
