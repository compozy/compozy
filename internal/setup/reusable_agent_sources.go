package setup

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	bundledagents "github.com/compozy/compozy/agents"
)

// ExtensionReusableAgentSource captures one declarative reusable-agent source resolved during extension discovery.
type ExtensionReusableAgentSource struct {
	ExtensionName   string
	ExtensionSource string
	ManifestPath    string
	Pattern         string
	ResolvedPath    string
	SourceFS        fs.FS
	SourceDir       string
}

type extensionReusableAgentSource struct {
	Source        ExtensionReusableAgentSource
	ReusableAgent ReusableAgent
}

// ListExtensionReusableAgents enumerates reusable agents declared by enabled extensions.
func ListExtensionReusableAgents(sources []ExtensionReusableAgentSource) ([]ReusableAgent, error) {
	loaded, err := loadExtensionReusableAgentSources(sources)
	if err != nil {
		return nil, err
	}

	agents := make([]ReusableAgent, 0, len(loaded))
	for i := range loaded {
		agents = append(agents, loaded[i].ReusableAgent)
	}
	return agents, nil
}

// PreviewReusableAgentInstall resolves the on-disk install plan for reusable agents.
func PreviewReusableAgentInstall(cfg ReusableAgentInstallConfig) ([]ReusableAgentPreviewItem, error) {
	env, err := resolveEnvironment(cfg.ResolverOptions)
	if err != nil {
		return nil, err
	}

	items := make([]ReusableAgentPreviewItem, 0, len(cfg.ReusableAgents))
	root := reusableAgentsInstallRoot(env, cfg.Global)
	for i := range cfg.ReusableAgents {
		targetPath, err := resolveReusableAgentInstallTargetPath(root, cfg.ReusableAgents[i])
		if err != nil {
			return nil, err
		}
		items = append(items, ReusableAgentPreviewItem{
			ReusableAgent: cfg.ReusableAgents[i],
			TargetPath:    targetPath,
			WillOverwrite: pathExists(targetPath),
		})
	}
	return items, nil
}

// InstallReusableAgents installs the provided reusable agents into the selected setup scope.
func InstallReusableAgents(
	cfg ReusableAgentInstallConfig,
) ([]ReusableAgentSuccessItem, []ReusableAgentFailureItem, error) {
	env, err := resolveEnvironment(cfg.ResolverOptions)
	if err != nil {
		return nil, nil, err
	}
	return installReusableAgents(reusableAgentsInstallRoot(env, cfg.Global), cfg.ReusableAgents)
}

// VerifyReusableAgents checks whether reusable agents are installed and current.
func VerifyReusableAgents(cfg ReusableAgentVerifyConfig) (ReusableAgentVerifyResult, error) {
	env, err := resolveEnvironment(cfg.ResolverOptions)
	if err != nil {
		return ReusableAgentVerifyResult{}, err
	}

	projectEntries, err := reusableAgentVerificationEntries(cfg.ReusableAgents, reusableAgentsInstallRoot(env, false))
	if err != nil {
		return ReusableAgentVerifyResult{}, err
	}
	globalEntries, err := reusableAgentVerificationEntries(cfg.ReusableAgents, reusableAgentsInstallRoot(env, true))
	if err != nil {
		return ReusableAgentVerifyResult{}, err
	}
	scope, entries := selectReusableAgentVerificationEntries(projectEntries, globalEntries, cfg.ScopeHint)

	verified := make([]VerifiedReusableAgent, 0, len(entries))
	for i := range entries {
		entry := entries[i]
		verifiedEntry := VerifiedReusableAgent{
			ReusableAgent: entry.Entry.ReusableAgent,
			TargetPath:    entry.Entry.TargetPath,
		}
		if entry.Scope == InstallScopeUnknown || !pathExists(entry.Entry.TargetPath) {
			verifiedEntry.State = VerifyStateMissing
			verified = append(verified, verifiedEntry)
			continue
		}

		resolvedPath := resolveInstalledPath(entry.Entry.TargetPath)
		verifiedEntry.ResolvedPath = resolvedPath

		sourceFS, sourceDir, err := resolveReusableAgentSource(entry.Entry.ReusableAgent)
		if err != nil {
			return ReusableAgentVerifyResult{}, fmt.Errorf(
				"verify reusable agent %q: %w",
				entry.Entry.ReusableAgent.Name,
				err,
			)
		}
		drift, drifted, err := compareInstalledDirectory(sourceFS, sourceDir, resolvedPath, "reusable agent")
		if err != nil {
			return ReusableAgentVerifyResult{}, fmt.Errorf(
				"verify reusable agent %q: %w",
				entry.Entry.ReusableAgent.Name,
				err,
			)
		}
		if drifted {
			verifiedEntry.State = VerifyStateDrifted
			verifiedEntry.Drift = drift
			verified = append(verified, verifiedEntry)
			continue
		}

		verifiedEntry.State = VerifyStateCurrent
		verified = append(verified, verifiedEntry)
	}

	return ReusableAgentVerifyResult{Scope: scope, Agents: verified}, nil
}

type reusableAgentVerificationEntry struct {
	ReusableAgent ReusableAgent
	TargetPath    string
}

type selectedReusableAgentVerificationEntry struct {
	Scope InstallScope
	Entry reusableAgentVerificationEntry
}

func reusableAgentVerificationEntries(
	reusableAgents []ReusableAgent,
	root string,
) ([]reusableAgentVerificationEntry, error) {
	entries := make([]reusableAgentVerificationEntry, 0, len(reusableAgents))
	for i := range reusableAgents {
		targetPath, err := resolveReusableAgentInstallTargetPath(root, reusableAgents[i])
		if err != nil {
			return nil, err
		}
		entries = append(entries, reusableAgentVerificationEntry{
			ReusableAgent: reusableAgents[i],
			TargetPath:    targetPath,
		})
	}
	return entries, nil
}

func selectReusableAgentVerificationEntries(
	projectEntries []reusableAgentVerificationEntry,
	globalEntries []reusableAgentVerificationEntry,
	scopeHint InstallScope,
) (InstallScope, []selectedReusableAgentVerificationEntry) {
	switch scopeHint {
	case InstallScopeProject:
		return InstallScopeProject, wrapReusableAgentVerificationEntries(InstallScopeProject, projectEntries)
	case InstallScopeGlobal:
		return InstallScopeGlobal, wrapReusableAgentVerificationEntries(InstallScopeGlobal, globalEntries)
	}

	scope := inferredReusableAgentVerificationScope(projectEntries, globalEntries)
	selected := make([]selectedReusableAgentVerificationEntry, 0, len(projectEntries))
	for i := range projectEntries {
		switch {
		case pathExists(projectEntries[i].TargetPath):
			selected = append(selected, selectedReusableAgentVerificationEntry{
				Scope: InstallScopeProject,
				Entry: projectEntries[i],
			})
		case pathExists(globalEntries[i].TargetPath):
			selected = append(selected, selectedReusableAgentVerificationEntry{
				Scope: InstallScopeGlobal,
				Entry: globalEntries[i],
			})
		default:
			entry := projectEntries[i]
			if scope == InstallScopeGlobal {
				entry = globalEntries[i]
			}
			selected = append(selected, selectedReusableAgentVerificationEntry{
				Scope: scope,
				Entry: entry,
			})
		}
	}

	return scope, selected
}

func wrapReusableAgentVerificationEntries(
	scope InstallScope,
	entries []reusableAgentVerificationEntry,
) []selectedReusableAgentVerificationEntry {
	selected := make([]selectedReusableAgentVerificationEntry, 0, len(entries))
	for i := range entries {
		selected = append(selected, selectedReusableAgentVerificationEntry{
			Scope: scope,
			Entry: entries[i],
		})
	}
	return selected
}

func inferredReusableAgentVerificationScope(
	projectEntries []reusableAgentVerificationEntry,
	globalEntries []reusableAgentVerificationEntry,
) InstallScope {
	switch {
	case hasAnyInstalledReusableAgent(projectEntries):
		return InstallScopeProject
	case hasAnyInstalledReusableAgent(globalEntries):
		return InstallScopeGlobal
	default:
		return InstallScopeUnknown
	}
}

func hasAnyInstalledReusableAgent(entries []reusableAgentVerificationEntry) bool {
	for i := range entries {
		if pathExists(entries[i].TargetPath) {
			return true
		}
	}
	return false
}

func reusableAgentsInstallRoot(env resolvedEnvironment, global bool) string {
	if global {
		return filepath.Join(env.homeDir, reusableAgentsInstallDir)
	}
	return filepath.Join(env.cwd, reusableAgentsInstallDir)
}

func installReusableAgents(
	root string,
	reusableAgents []ReusableAgent,
) ([]ReusableAgentSuccessItem, []ReusableAgentFailureItem, error) {
	successes := make([]ReusableAgentSuccessItem, 0, len(reusableAgents))
	failures := make([]ReusableAgentFailureItem, 0)
	for i := range reusableAgents {
		success, failure := installReusableAgent(root, reusableAgents[i])
		if failure != nil {
			failures = append(failures, *failure)
			continue
		}
		successes = append(successes, *success)
	}

	return successes, failures, nil
}

func installReusableAgent(
	root string,
	reusableAgent ReusableAgent,
) (*ReusableAgentSuccessItem, *ReusableAgentFailureItem) {
	targetPath, err := resolveReusableAgentInstallTargetPath(root, reusableAgent)
	if err != nil {
		return nil, reusableAgentFailure(reusableAgent, "", err)
	}

	tempTarget, err := prepareReusableAgentInstallTarget(root, reusableAgentsInstallDirName(reusableAgent))
	if err != nil {
		return nil, reusableAgentFailure(reusableAgent, targetPath, err)
	}

	sourceFS, sourceDir, err := resolveReusableAgentSource(reusableAgent)
	if err != nil {
		return nil, reusableAgentFailure(reusableAgent, targetPath, cleanupReusableAgentTempTarget(tempTarget, err))
	}
	if err := copyReusableAgentBundleDirectory(sourceFS, sourceDir, tempTarget, "reusable agent"); err != nil {
		return nil, reusableAgentFailure(reusableAgent, targetPath, cleanupReusableAgentTempTarget(tempTarget, err))
	}
	if err := replaceReusableAgentInstallTarget(tempTarget, targetPath); err != nil {
		return nil, reusableAgentFailure(reusableAgent, targetPath, cleanupReusableAgentTempTarget(tempTarget, err))
	}

	return &ReusableAgentSuccessItem{
		ReusableAgent: reusableAgent,
		Path:          targetPath,
	}, nil
}

func reusableAgentFailure(
	reusableAgent ReusableAgent,
	path string,
	err error,
) *ReusableAgentFailureItem {
	return &ReusableAgentFailureItem{
		ReusableAgent: reusableAgent,
		Path:          path,
		Error:         err.Error(),
	}
}

func cleanupReusableAgentTempTarget(tempTarget string, cause error) error {
	cleanupErr := removeReusableAgentPath(tempTarget)
	if cleanupErr == nil {
		return cause
	}
	return errors.Join(
		cause,
		fmt.Errorf("cleanup reusable agent staging directory %s: %w", tempTarget, cleanupErr),
	)
}

func loadExtensionReusableAgentSources(
	sources []ExtensionReusableAgentSource,
) ([]extensionReusableAgentSource, error) {
	if len(sources) == 0 {
		return nil, nil
	}

	loaded := make([]extensionReusableAgentSource, 0, len(sources))
	for _, source := range sources {
		sourceFS, sourceDir, err := resolveExtensionReusableAgentSource(source)
		if err != nil {
			return nil, err
		}
		reusableAgent, err := parseReusableAgent(sourceFS, sourceDir)
		if err != nil {
			return nil, fmt.Errorf(
				"load extension reusable agent %q from %q: %w",
				source.ExtensionName,
				source.ResolvedPath,
				err,
			)
		}

		reusableAgent.Origin = AssetOriginExtension
		reusableAgent.ExtensionName = source.ExtensionName
		reusableAgent.ExtensionSource = source.ExtensionSource
		reusableAgent.ManifestPath = source.ManifestPath
		reusableAgent.ResolvedPath = source.ResolvedPath
		reusableAgent.SourceFS = sourceFS
		reusableAgent.SourceDir = sourceDir

		loaded = append(loaded, extensionReusableAgentSource{
			Source:        source,
			ReusableAgent: reusableAgent,
		})
	}

	slices.SortFunc(loaded, compareExtensionReusableAgentSource)
	return loaded, nil
}

func resolveExtensionReusableAgentSource(source ExtensionReusableAgentSource) (fs.FS, string, error) {
	if source.SourceFS != nil && strings.TrimSpace(source.SourceDir) != "" {
		return source.SourceFS, strings.TrimSpace(source.SourceDir), nil
	}

	resolvedPath := strings.TrimSpace(source.ResolvedPath)
	if resolvedPath == "" {
		return nil, "", fmt.Errorf("extension reusable agent source path is required")
	}
	resolvedPath = filepath.Clean(resolvedPath)
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return nil, "", fmt.Errorf("stat extension reusable agent %q: %w", resolvedPath, err)
	}
	if !info.IsDir() {
		return nil, "", fmt.Errorf("extension reusable agent %q is not a directory", resolvedPath)
	}

	parentDir := filepath.Dir(resolvedPath)
	sourceDir := filepath.Base(resolvedPath)
	return os.DirFS(parentDir), sourceDir, nil
}

func compareExtensionReusableAgentSource(left, right extensionReusableAgentSource) int {
	if diff := strings.Compare(left.ReusableAgent.Name, right.ReusableAgent.Name); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.Source.ExtensionName, right.Source.ExtensionName); diff != 0 {
		return diff
	}
	if diff := strings.Compare(left.Source.ManifestPath, right.Source.ManifestPath); diff != 0 {
		return diff
	}
	return strings.Compare(left.Source.ResolvedPath, right.Source.ResolvedPath)
}

func resolveReusableAgentSource(reusableAgent ReusableAgent) (fs.FS, string, error) {
	if reusableAgent.SourceFS != nil && strings.TrimSpace(reusableAgent.SourceDir) != "" {
		return reusableAgent.SourceFS, strings.TrimSpace(reusableAgent.SourceDir), nil
	}
	if reusableAgent.Origin == AssetOriginBundled && strings.TrimSpace(reusableAgent.Directory) != "" {
		return bundledagents.FS, reusableAgent.Directory, nil
	}
	if strings.TrimSpace(reusableAgent.ResolvedPath) != "" {
		info, err := os.Stat(reusableAgent.ResolvedPath)
		if err != nil {
			return nil, "", fmt.Errorf("stat reusable agent source %q: %w", reusableAgent.ResolvedPath, err)
		}
		if !info.IsDir() {
			return nil, "", fmt.Errorf("reusable agent source %q is not a directory", reusableAgent.ResolvedPath)
		}
		parentDir := filepath.Dir(reusableAgent.ResolvedPath)
		sourceDir := filepath.Base(reusableAgent.ResolvedPath)
		return os.DirFS(parentDir), sourceDir, nil
	}
	return nil, "", fmt.Errorf("reusable agent %q does not declare a source directory", reusableAgent.Name)
}

func reusableAgentsInstallDirName(reusableAgent ReusableAgent) string {
	return filepath.Base(strings.TrimSpace(reusableAgent.Name))
}

func resolveReusableAgentInstallTargetPath(root string, reusableAgent ReusableAgent) (string, error) {
	if err := validateReusableAgentName(reusableAgent.Name); err != nil {
		return "", fmt.Errorf("resolve reusable agent target path for %q: %w", reusableAgent.Name, err)
	}

	targetPath := filepath.Join(root, reusableAgentsInstallDirName(reusableAgent))
	if !isPathSafe(root, targetPath) {
		return "", fmt.Errorf(
			"resolve reusable agent target path for %q: path escaped base directory",
			reusableAgent.Name,
		)
	}
	return targetPath, nil
}
