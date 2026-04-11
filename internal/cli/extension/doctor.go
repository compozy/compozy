package extension

import (
	"context"
	"fmt"
	"io"
	"slices"
	"strings"

	extensions "github.com/compozy/compozy/internal/core/extension"
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

	report := buildDoctorReport(ctx, result)
	output := renderDoctorReport(report)
	if _, err := io.WriteString(cmd.OutOrStdout(), output); err != nil {
		return fmt.Errorf("write extension doctor report: %w", err)
	}
	if len(report.Errors) > 0 {
		return fmt.Errorf("extension doctor found %d error(s)", len(report.Errors))
	}
	return nil
}

func buildDoctorReport(ctx context.Context, result extensions.DiscoveryResult) doctorReport {
	report := doctorReport{
		Infos: []string{
			"Skill-pack drift check is not implemented yet (task 13 placeholder).",
			"Provider overlay drift check is not implemented yet (task 13 placeholder).",
		},
	}

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

	report.Warnings = append(report.Warnings, priorityTieWarnings(result.Extensions)...)
	slices.Sort(report.Errors)
	slices.Sort(report.Warnings)
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
