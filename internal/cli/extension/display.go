package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/spf13/cobra"
)

func newListCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:          "list",
		Short:        "List discovered extensions across bundled, user, and workspace scopes",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runListCommand(cmd, deps)
		},
	}
}

func runListCommand(cmd *cobra.Command, deps commandDeps) error {
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

	content, err := renderList(result)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), content); err != nil {
		return fmt.Errorf("write extension list: %w", err)
	}
	return nil
}

func renderList(result extensions.DiscoveryResult) (string, error) {
	active := effectiveManifestPaths(result)

	var buf bytes.Buffer
	writer := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
	if _, err := fmt.Fprintln(writer, "NAME\tVERSION\tSOURCE\tENABLED\tACTIVE\tCAPABILITIES"); err != nil {
		return "", fmt.Errorf("write list header: %w", err)
	}
	for index := range result.Discovered {
		entry := result.Discovered[index]
		isActive := entry.Enabled && active[entry.ManifestPath]
		if _, err := fmt.Fprintf(
			writer,
			"%s\t%s\t%s\t%t\t%t\t%s\n",
			entry.Ref.Name,
			entry.Manifest.Extension.Version,
			entry.Ref.Source,
			entry.Enabled,
			isActive,
			renderCapabilities(entry.Manifest.Security.Capabilities),
		); err != nil {
			return "", fmt.Errorf("write list row: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return "", fmt.Errorf("flush extension list: %w", err)
	}
	return buf.String(), nil
}

func newInspectCommand(deps commandDeps) *cobra.Command {
	return &cobra.Command{
		Use:          "inspect <name>",
		Short:        "Inspect one effective extension declaration",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInspectCommand(cmd, deps, args[0])
		},
	}
}

func runInspectCommand(cmd *cobra.Command, deps commandDeps, rawName string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	name, err := normalizeExtensionName(rawName)
	if err != nil {
		return err
	}

	env, err := deps.resolveEnv(ctx)
	if err != nil {
		return err
	}

	result, err := deps.discoverAll(ctx, env)
	if err != nil {
		return err
	}

	entry, ok := findEffectiveExtension(result, name)
	if !ok {
		return fmt.Errorf("extension %q not found", name)
	}

	content, err := renderInspect(ctx, result, entry)
	if err != nil {
		return err
	}
	if _, err := io.WriteString(cmd.OutOrStdout(), content); err != nil {
		return fmt.Errorf("write extension inspection: %w", err)
	}
	return nil
}

func renderInspect(
	ctx context.Context,
	result extensions.DiscoveryResult,
	entry extensions.DiscoveredExtension,
) (string, error) {
	manifestJSON, err := json.MarshalIndent(entry.Manifest, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal extension manifest: %w", err)
	}

	var buf strings.Builder
	active := effectiveManifestPaths(result)[entry.ManifestPath] && entry.Enabled

	fmt.Fprintf(&buf, "Name: %s\n", entry.Ref.Name)
	fmt.Fprintf(&buf, "Version: %s\n", entry.Manifest.Extension.Version)
	fmt.Fprintf(&buf, "Source: %s\n", entry.Ref.Source)
	fmt.Fprintf(&buf, "Enabled: %t\n", entry.Enabled)
	fmt.Fprintf(&buf, "Active: %t\n", active)
	fmt.Fprintf(&buf, "Manifest path: %s\n", entry.ManifestPath)
	fmt.Fprintf(&buf, "Extension dir: %s\n", entry.ExtensionDir)
	fmt.Fprintf(&buf, "Capabilities: %s\n", renderCapabilities(entry.Manifest.Security.Capabilities))

	buf.WriteString("\nActive hooks:\n")
	hooks := renderHooks(entry.Manifest.Hooks)
	for _, line := range hooks {
		fmt.Fprintf(&buf, "- %s\n", line)
	}

	buf.WriteString("\nOverrides:\n")
	records := overrideRecordsForName(result, entry.Ref.Name)
	if len(records) == 0 {
		buf.WriteString("- (none)\n")
	} else {
		for index := range records {
			record := records[index]
			fmt.Fprintf(
				&buf,
				"- winner=%s@%s loser=%s@%s reason=%s\n",
				record.Winner.Source,
				record.Winner.ManifestPath,
				record.Loser.Source,
				record.Loser.ManifestPath,
				record.Reason,
			)
		}
	}

	appendDiscoveryFailureNotes(ctx, &buf, result, entry.Ref.Name)

	buf.WriteString("\nManifest:\n")
	buf.Write(manifestJSON)
	buf.WriteString("\n")
	return buf.String(), nil
}

func appendDiscoveryFailureNotes(
	_ context.Context,
	buf *strings.Builder,
	result extensions.DiscoveryResult,
	name string,
) {
	matchingFailures := make([]extensions.DiscoveryFailure, 0)
	for _, failure := range result.Failures {
		if strings.Contains(strings.ToLower(failure.ExtensionDir), strings.ToLower(strings.TrimSpace(name))) {
			matchingFailures = append(matchingFailures, failure)
		}
	}
	if len(matchingFailures) == 0 {
		return
	}

	buf.WriteString("\nDiscovery failures:\n")
	for _, failure := range matchingFailures {
		fmt.Fprintf(
			buf,
			"- source=%s manifest=%s error=%v\n",
			failure.Source,
			failure.ManifestPath,
			failure.Err,
		)
	}
}

func renderHooks(hooks []extensions.HookDeclaration) []string {
	if len(hooks) == 0 {
		return []string{"(none)"}
	}

	lines := make([]string, 0, len(hooks))
	for _, hook := range hooks {
		lines = append(
			lines,
			fmt.Sprintf(
				"%s priority=%d required=%t timeout=%s",
				hook.Event,
				hook.Priority,
				hook.Required,
				hook.Timeout,
			),
		)
	}
	return lines
}

func effectiveManifestPaths(result extensions.DiscoveryResult) map[string]bool {
	paths := make(map[string]bool, len(result.Extensions))
	for index := range result.Extensions {
		entry := result.Extensions[index]
		paths[entry.ManifestPath] = true
	}
	return paths
}
