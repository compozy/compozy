package extension

import (
	"context"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

type doctorReport struct {
	Errors   []string
	Warnings []string
	Infos    []string
}

func newDoctorCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:          "doctor",
		Short:        "Validate extension manifests and report local health warnings",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runDoctorCommand(cmd, deps)
		},
	}
}

func runDoctorCommand(cmd *cobra.Command, deps commandDeps) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	env, err := deps.resolveEnv(ctx)
	if err != nil {
		return err
	}

	result, err := deps.discoverAll(ctx, env)
	if err != nil {
		return err
	}

	report := buildDoctorReport(ctx, result, buildResolverOptions(env))
	output := renderDoctorReport(report)
	if _, err := io.WriteString(cmd.OutOrStdout(), output); err != nil {
		return fmt.Errorf("write extension doctor report: %w", err)
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("extension doctor found %d error(s)", len(report.Errors))
	}
	return nil
}

func buildDoctorReport(
	ctx context.Context,
	result extensions.DiscoveryResult,
	resolver setup.ResolverOptions,
) doctorReport {
	report := doctorReport{}
	var effectiveCatalog setup.EffectiveCatalog

	for _, failure := range result.Failures {
		report.Errors = append(
			report.Errors,
			fmt.Sprintf("[%s] %s: %v", failure.Source, failure.ManifestPath, failure.Err),
		)
	}

	for index := range result.Discovered {
		entry := result.Discovered[index]
		if err := extensions.ValidateManifest(ctx, entry.Manifest); err != nil {
			report.Errors = append(
				report.Errors,
				fmt.Sprintf("[%s] %s: %v", entry.Ref.Source, entry.ManifestPath, err),
			)
		}

		report.Warnings = append(report.Warnings, unusedCapabilityWarnings(entry)...)
	}

	report.Warnings = append(report.Warnings, providerConflictWarnings(result.Extensions, result.Providers)...)
	report.Warnings = append(report.Warnings, priorityTieWarnings(result.Extensions)...)
	catalog, err := buildDoctorEffectiveCatalog(result)
	if err != nil {
		report.Warnings = append(report.Warnings, fmt.Sprintf("setup asset health check skipped: %v", err))
	} else {
		effectiveCatalog = catalog
		report.Warnings = append(report.Warnings, setupAssetConflictWarnings(effectiveCatalog.Conflicts)...)
		report.Warnings = append(report.Warnings, skillPackDriftWarnings(effectiveCatalog, resolver)...)
		report.Warnings = append(report.Warnings, reusableAgentDriftWarnings(effectiveCatalog, resolver)...)
	}
	report.Infos = append(report.Infos, overrideInfos(result.Overrides)...)
	slices.Sort(report.Errors)
	slices.Sort(report.Warnings)
	slices.Sort(report.Infos)
	if len(report.Infos) == 0 {
		report.Infos = append(report.Infos, "No extension override records detected.")
	}
	return report
}

func renderDoctorReport(report doctorReport) string {
	var buf strings.Builder

	fmt.Fprintf(
		&buf,
		"Doctor summary: %d error(s), %d warning(s)\n",
		len(report.Errors),
		len(report.Warnings),
	)
	writeDoctorSection(&buf, "Errors", report.Errors)
	writeDoctorSection(&buf, "Warnings", report.Warnings)
	writeDoctorSection(&buf, "Info", report.Infos)

	return buf.String()
}

func writeDoctorSection(buf *strings.Builder, title string, items []string) {
	fmt.Fprintf(buf, "\n%s:\n", title)
	if len(items) == 0 {
		buf.WriteString("- (none)\n")
		return
	}
	for _, item := range items {
		fmt.Fprintf(buf, "- %s\n", item)
	}
}

func priorityTieWarnings(entries []extensions.DiscoveredExtension) []string {
	type tieKey struct {
		hook     extensions.HookName
		priority int
	}

	groups := make(map[tieKey][]string)
	for index := range entries {
		entry := entries[index]
		if !entry.Enabled {
			continue
		}
		for _, hook := range entry.Manifest.Hooks {
			key := tieKey{hook: hook.Event, priority: hook.Priority}
			groups[key] = append(groups[key], entry.Ref.Name)
		}
	}

	warnings := make([]string, 0)
	for key, names := range groups {
		names = uniqueSortedStrings(names)
		if len(names) < 2 {
			continue
		}

		warnings = append(
			warnings,
			fmt.Sprintf(
				"priority tie on %s at %d across %s",
				key.hook,
				key.priority,
				strings.Join(names, ", "),
			),
		)
	}
	return warnings
}

func unusedCapabilityWarnings(entry extensions.DiscoveredExtension) []string {
	warnings := make([]string, 0)
	for _, capability := range sortedCapabilities(entry.Manifest.Security.Capabilities) {
		if capabilityHasManifestEvidence(entry.Manifest, capability) {
			continue
		}

		warnings = append(
			warnings,
			fmt.Sprintf(
				"extension %q declares capability %q without a matching hook/resource/provider/subprocess signal in the manifest",
				entry.Ref.Name,
				capability,
			),
		)
	}
	return warnings
}

func capabilityHasManifestEvidence(manifest *extensions.Manifest, capability extensions.Capability) bool {
	if manifest == nil {
		return false
	}

	switch capability {
	case extensions.CapabilityPlanMutate:
		return hasHookPrefix(manifest, "plan.")
	case extensions.CapabilityPromptMutate:
		return hasHookPrefix(manifest, "prompt.")
	case extensions.CapabilityAgentMutate:
		return hasHookPrefix(manifest, "agent.")
	case extensions.CapabilityJobMutate:
		return hasHookPrefix(manifest, "job.")
	case extensions.CapabilityRunMutate:
		return hasHookPrefix(manifest, "run.")
	case extensions.CapabilityReviewMutate:
		return hasHookPrefix(manifest, "review.")
	case extensions.CapabilityArtifactsWrite:
		return hasHookPrefix(manifest, "artifact.") || manifest.Subprocess != nil
	case extensions.CapabilityProvidersRegister:
		return len(manifest.Providers.IDE)+len(manifest.Providers.Review)+len(manifest.Providers.Model) > 0
	case extensions.CapabilitySkillsShip:
		return len(manifest.Resources.Skills) > 0
	case extensions.CapabilityAgentsShip:
		return len(manifest.Resources.Agents) > 0
	case extensions.CapabilityEventsRead,
		extensions.CapabilityEventsPublish,
		extensions.CapabilityArtifactsRead,
		extensions.CapabilityTasksRead,
		extensions.CapabilityTasksCreate,
		extensions.CapabilityRunsStart,
		extensions.CapabilityMemoryRead,
		extensions.CapabilityMemoryWrite,
		extensions.CapabilitySubprocessSpawn,
		extensions.CapabilityNetworkEgress:
		return manifest.Subprocess != nil
	default:
		return true
	}
}

func hasHookPrefix(manifest *extensions.Manifest, prefix string) bool {
	for _, hook := range manifest.Hooks {
		if strings.HasPrefix(string(hook.Event), prefix) {
			return true
		}
	}
	return false
}

func uniqueSortedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	slices.Sort(result)
	return result
}

func buildResolverOptions(env commandEnv) setup.ResolverOptions {
	return setup.ResolverOptions{
		CWD:             env.workspaceRoot,
		HomeDir:         env.homeDir,
		CodeXHome:       strings.TrimSpace(os.Getenv("CODEX_HOME")),
		ClaudeConfigDir: strings.TrimSpace(os.Getenv("CLAUDE_CONFIG_DIR")),
		XDGConfigHome:   strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")),
	}
}

func overrideInfos(records []extensions.OverrideRecord) []string {
	infos := make([]string, 0, len(records))
	for i := range records {
		record := &records[i]
		infos = append(infos, fmt.Sprintf(
			"extension %q from %s overrides %s declaration from %s (%s)",
			record.Name,
			record.Winner.Source,
			record.Loser.Source,
			record.Loser.ManifestPath,
			record.Reason,
		))
	}
	return infos
}

func providerConflictWarnings(
	entries []extensions.DiscoveredExtension,
	providers extensions.DeclaredProviders,
) []string {
	enabled := enabledExtensionNames(entries)
	conflicts := make([]string, 0)
	conflicts = append(conflicts, appendProviderConflictWarnings("ide", providers.IDE, enabled)...)
	conflicts = append(conflicts, appendProviderConflictWarnings("review", providers.Review, enabled)...)
	conflicts = append(conflicts, appendProviderConflictWarnings("model", providers.Model, enabled)...)
	return conflicts
}

func enabledExtensionNames(entries []extensions.DiscoveredExtension) map[string]struct{} {
	enabled := make(map[string]struct{}, len(entries))
	for i := range entries {
		entry := &entries[i]
		if !entry.Enabled {
			continue
		}
		enabled[entry.Ref.Name] = struct{}{}
	}
	return enabled
}

func appendProviderConflictWarnings(
	category string,
	providers []extensions.DeclaredProvider,
	enabled map[string]struct{},
) []string {
	grouped := make(map[string][]string)
	for i := range providers {
		declared := providers[i]
		if _, ok := enabled[declared.Extension.Name]; !ok {
			continue
		}
		name := strings.TrimSpace(strings.ToLower(declared.Name))
		grouped[name] = append(grouped[name], declared.Extension.Name)
	}

	warnings := make([]string, 0)
	for name, owners := range grouped {
		owners = uniqueSortedStrings(owners)
		if len(owners) < 2 {
			continue
		}
		warnings = append(
			warnings,
			fmt.Sprintf(
				"provider overlay conflict on %s provider %q across %s",
				category,
				name,
				strings.Join(owners, ", "),
			),
		)
	}
	return warnings
}

func skillPackDriftWarnings(
	catalog setup.EffectiveCatalog,
	resolver setup.ResolverOptions,
) []string {
	packs := setup.ExtensionSkillPackSources(catalog.Skills)
	if len(packs) == 0 {
		return nil
	}

	agents, err := setup.DetectInstalledAgents(resolver)
	if err != nil {
		return []string{fmt.Sprintf("extension skill drift check skipped: %v", err)}
	}
	if len(agents) == 0 {
		return []string{"extension skill drift check skipped: no supported agents detected"}
	}

	warnings := make([]string, 0)
	for _, agent := range agents {
		result, err := setup.VerifyExtensionSkillPacks(setup.ExtensionVerifyConfig{
			ResolverOptions: resolver,
			Packs:           packs,
			AgentName:       agent.Name,
		})
		if err != nil {
			warnings = append(
				warnings,
				fmt.Sprintf("extension skill drift check failed for %s: %v", agent.DisplayName, err),
			)
			continue
		}
		if !result.HasMissing() && !result.HasDrift() {
			continue
		}

		parts := make([]string, 0, 2)
		if result.HasMissing() {
			parts = append(parts, "missing "+strings.Join(result.MissingSkillNames(), ", "))
		}
		if result.HasDrift() {
			parts = append(parts, "drifted "+strings.Join(result.DriftedSkillNames(), ", "))
		}
		warnings = append(
			warnings,
			fmt.Sprintf(
				"extension skill-pack drift for %s (%s scope): %s",
				result.Agent.DisplayName,
				result.Scope,
				strings.Join(parts, "; "),
			),
		)
	}
	return warnings
}

func reusableAgentDriftWarnings(catalog setup.EffectiveCatalog, resolver setup.ResolverOptions) []string {
	reusableAgents := extensionReusableAgents(catalog.ReusableAgents)
	if len(reusableAgents) == 0 {
		return nil
	}

	result, err := setup.VerifyReusableAgents(resolver, reusableAgents)
	if err != nil {
		return []string{fmt.Sprintf("extension reusable-agent drift check failed: %v", err)}
	}
	if !result.HasMissing() && !result.HasDrift() {
		return nil
	}

	parts := make([]string, 0, 2)
	if result.HasMissing() {
		parts = append(parts, "missing "+strings.Join(result.MissingReusableAgentNames(), ", "))
	}
	if result.HasDrift() {
		parts = append(parts, "drifted "+strings.Join(result.DriftedReusableAgentNames(), ", "))
	}

	return []string{
		fmt.Sprintf(
			"extension reusable-agent drift (global scope): %s",
			strings.Join(parts, "; "),
		),
	}
}

func setupAssetConflictWarnings(conflicts []setup.CatalogConflict) []string {
	if len(conflicts) == 0 {
		return nil
	}

	warnings := make([]string, 0, len(conflicts))
	for i := range conflicts {
		conflict := &conflicts[i]
		subject := string(conflict.Kind)
		switch conflict.Resolution {
		case setup.CatalogConflictCoreWins:
			warnings = append(
				warnings,
				fmt.Sprintf(
					"setup %s conflict on %q: ignored %s because the core %s wins",
					subject,
					conflict.Name,
					doctorAssetSourceLabel(conflict.Ignored),
					subject,
				),
			)
		case setup.CatalogConflictExtensionPrecedence:
			warnings = append(
				warnings,
				fmt.Sprintf(
					"setup %s conflict on %q: ignored %s because %s wins by precedence",
					subject,
					conflict.Name,
					doctorAssetSourceLabel(conflict.Ignored),
					doctorAssetSourceLabel(conflict.Winner),
				),
			)
		}
	}
	return warnings
}

func toSetupSkillPackSources(packs []extensions.DeclaredSkillPack) []setup.SkillPackSource {
	if len(packs) == 0 {
		return nil
	}

	result := make([]setup.SkillPackSource, 0, len(packs))
	for i := range packs {
		pack := &packs[i]
		result = append(result, setup.SkillPackSource{
			ExtensionName:   pack.Extension.Name,
			ExtensionSource: string(pack.Extension.Source),
			ManifestPath:    pack.ManifestPath,
			Pattern:         pack.Pattern,
			ResolvedPath:    pack.ResolvedPath,
			SourceFS:        pack.SourceFS,
			SourceDir:       pack.SourceDir,
		})
	}
	return result
}

func toSetupReusableAgentSources(
	agents []extensions.DeclaredReusableAgent,
) []setup.ExtensionReusableAgentSource {
	if len(agents) == 0 {
		return nil
	}

	result := make([]setup.ExtensionReusableAgentSource, 0, len(agents))
	for i := range agents {
		reusableAgent := &agents[i]
		result = append(result, setup.ExtensionReusableAgentSource{
			ExtensionName:   reusableAgent.Extension.Name,
			ExtensionSource: string(reusableAgent.Extension.Source),
			ManifestPath:    reusableAgent.ManifestPath,
			Pattern:         reusableAgent.Pattern,
			ResolvedPath:    reusableAgent.ResolvedPath,
			SourceFS:        reusableAgent.SourceFS,
			SourceDir:       reusableAgent.SourceDir,
		})
	}
	return result
}

func buildDoctorEffectiveCatalog(result extensions.DiscoveryResult) (setup.EffectiveCatalog, error) {
	bundledSkills, err := setup.ListBundledSkills()
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}
	bundledReusableAgents, err := setup.ListBundledReusableAgents()
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}
	extensionSkills, err := setup.ListExtensionSkills(toSetupSkillPackSources(result.SkillPacks.Packs))
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}
	extensionReusableAgents, err := setup.ListExtensionReusableAgents(
		toSetupReusableAgentSources(result.ReusableAgents.Agents),
	)
	if err != nil {
		return setup.EffectiveCatalog{}, err
	}

	return setup.BuildEffectiveCatalog(
		bundledSkills,
		extensionSkills,
		bundledReusableAgents,
		extensionReusableAgents,
	), nil
}

func extensionReusableAgents(all []setup.ReusableAgent) []setup.ReusableAgent {
	if len(all) == 0 {
		return nil
	}

	reusableAgents := make([]setup.ReusableAgent, 0, len(all))
	for i := range all {
		if all[i].Origin != setup.AssetOriginExtension {
			continue
		}
		reusableAgents = append(reusableAgents, all[i])
	}
	return reusableAgents
}

func doctorAssetSourceLabel(ref setup.AssetRef) string {
	if ref.Origin == setup.AssetOriginBundled {
		return "core"
	}

	parts := make([]string, 0, 2)
	if trimmedSource := strings.TrimSpace(ref.ExtensionSource); trimmedSource != "" {
		parts = append(parts, trimmedSource)
	}
	if trimmedName := strings.TrimSpace(ref.ExtensionName); trimmedName != "" {
		parts = append(parts, trimmedName)
	}
	if len(parts) == 0 {
		return "extension"
	}
	return strings.Join(parts, ":")
}
