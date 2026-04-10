package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	reusableagents "github.com/compozy/compozy/internal/core/agents"
	"github.com/compozy/compozy/internal/core/agents/mcpserver"
	"github.com/spf13/cobra"
)

type reusableAgentRegistryFactory func() *reusableagents.Registry

type agentsListCommandState struct {
	simpleCommandBase
	newRegistry reusableAgentRegistryFactory
}

type agentsInspectCommandState struct {
	simpleCommandBase
	newRegistry reusableAgentRegistryFactory
	name        string
}

type mcpServeCommandState struct {
	serverName      string
	loadHostContext func() (mcpserver.HostContext, error)
	serveStdio      func(context.Context, mcpserver.HostContext) error
}

type inspectAgentReport struct {
	Name            string
	Source          reusableagents.Source
	Status          string
	Metadata        reusableagents.Metadata
	Runtime         reusableagents.RuntimeDefaults
	MCP             *reusableagents.MCPConfig
	ValidationError error
}

func newAgentsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "agents",
		Short:        "Discover and inspect reusable agents",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Discover reusable agents from workspace and global scope.

Workspace agents live under .compozy/agents/<name>/ and override same-name agents from
~/.compozy/agents/<name>/ as a whole directory.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}

	cmd.AddCommand(
		newAgentsListCommand(),
		newAgentsInspectCommand(),
		newMCPServeCommand(),
	)
	return cmd
}

func newAgentsListCommand() *cobra.Command {
	state := &agentsListCommandState{newRegistry: func() *reusableagents.Registry { return reusableagents.New() }}
	return &cobra.Command{
		Use:          "list",
		Short:        "List resolved reusable agents",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Example: `  compozy agents list
  compozy agents inspect council`,
		RunE: state.run,
	}
}

func newAgentsInspectCommand() *cobra.Command {
	state := &agentsInspectCommandState{newRegistry: func() *reusableagents.Registry { return reusableagents.New() }}
	cmd := &cobra.Command{
		Use:          "inspect <name>",
		Short:        "Inspect one reusable agent and its validation status",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Example: `  compozy agents inspect council
  compozy agents inspect planner`,
		RunE: state.run,
	}
	return cmd
}

func newMCPServeCommand() *cobra.Command {
	state := &mcpServeCommandState{
		serverName:      reusableagents.ReservedMCPServerName,
		loadHostContext: mcpserver.LoadHostContextFromEnv,
		serveStdio: func(ctx context.Context, host mcpserver.HostContext) error {
			return mcpserver.ServeStdio(ctx, host)
		},
	}
	cmd := &cobra.Command{
		Use:          "mcp-serve",
		Short:        "Serve the reserved Compozy MCP server over stdio",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Hidden:       true,
		RunE:         state.run,
	}
	cmd.Flags().StringVar(
		&state.serverName,
		"server",
		reusableagents.ReservedMCPServerName,
		"Reserved MCP server to expose over stdio",
	)
	return cmd
}

func (s *agentsListCommandState) run(cmd *cobra.Command, _ []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return withExitCode(2, fmt.Errorf("load workspace root for %s: %w", cmd.CommandPath(), err))
	}

	catalog, err := s.registry().Discover(ctx, s.workspaceRoot)
	if err != nil {
		return withExitCode(2, fmt.Errorf("discover reusable agents: %w", err))
	}
	if err := writeAgentsListText(cmd.OutOrStdout(), catalog); err != nil {
		return withExitCode(2, fmt.Errorf("write agents list: %w", err))
	}
	return nil
}

func (s *agentsListCommandState) registry() *reusableagents.Registry {
	if s.newRegistry != nil {
		return s.newRegistry()
	}
	return reusableagents.New()
}

func (s *agentsInspectCommandState) run(cmd *cobra.Command, args []string) error {
	ctx, stop := signalCommandContext(cmd)
	defer stop()

	s.name = strings.TrimSpace(args[0])
	if s.name == "" {
		return withExitCode(2, errors.New("missing agent name"))
	}
	if err := s.loadWorkspaceRoot(ctx); err != nil {
		return withExitCode(2, fmt.Errorf("load workspace root for %s: %w", cmd.CommandPath(), err))
	}

	catalog, err := s.registry().Discover(ctx, s.workspaceRoot)
	if err != nil {
		return withExitCode(2, fmt.Errorf("discover reusable agents: %w", err))
	}

	report, err := buildInspectAgentReport(catalog, s.name)
	if err != nil {
		return withExitCode(2, err)
	}
	if err := writeInspectAgentText(cmd.OutOrStdout(), report); err != nil {
		return withExitCode(2, fmt.Errorf("write agent inspection report: %w", err))
	}
	if report.ValidationError != nil {
		return withExitCode(1, fmt.Errorf("reusable agent %q is invalid", report.Name))
	}
	return nil
}

func (s *agentsInspectCommandState) registry() *reusableagents.Registry {
	if s.newRegistry != nil {
		return s.newRegistry()
	}
	return reusableagents.New()
}

func (s *mcpServeCommandState) run(cmd *cobra.Command, _ []string) error {
	serverName := strings.TrimSpace(s.serverName)
	if serverName != reusableagents.ReservedMCPServerName {
		return withExitCode(
			2,
			fmt.Errorf(
				"unsupported reserved MCP server %q (expected %q)",
				serverName,
				reusableagents.ReservedMCPServerName,
			),
		)
	}

	loadHostContext := s.loadHostContext
	if loadHostContext == nil {
		loadHostContext = mcpserver.LoadHostContextFromEnv
	}
	host, err := loadHostContext()
	if err != nil {
		return withExitCode(2, fmt.Errorf("load reserved MCP server context: %w", err))
	}

	serveStdio := s.serveStdio
	if serveStdio == nil {
		serveStdio = func(ctx context.Context, host mcpserver.HostContext) error {
			return mcpserver.ServeStdio(ctx, host)
		}
	}

	ctx, stop := signalCommandContext(cmd)
	defer stop()
	if err := serveStdio(ctx, host); err != nil {
		return withExitCode(2, fmt.Errorf("serve reserved MCP server %q: %w", serverName, err))
	}
	return nil
}

func buildInspectAgentReport(catalog reusableagents.Catalog, name string) (inspectAgentReport, error) {
	normalized := strings.TrimSpace(name)
	for idx := range catalog.Agents {
		agent := catalog.Agents[idx]
		if agent.Name != normalized {
			continue
		}
		return inspectAgentReport{
			Name:     agent.Name,
			Source:   agent.Source,
			Status:   "valid",
			Metadata: agent.Metadata,
			Runtime:  agent.Runtime,
			MCP:      agent.MCP,
		}, nil
	}
	for _, problem := range catalog.Problems {
		if problem.Name != normalized {
			continue
		}
		return inspectAgentReport{
			Name:            displayProblemName(problem),
			Source:          problem.Source,
			Status:          "invalid",
			ValidationError: problem.Err,
		}, nil
	}

	available := availableAgentNames(catalog)
	if len(available) == 0 {
		return inspectAgentReport{}, fmt.Errorf(
			"reusable agent %q not found; no reusable agents are currently available. Run `compozy agents list` for details",
			normalized,
		)
	}
	return inspectAgentReport{}, fmt.Errorf(
		"reusable agent %q not found; available agents: %s. Run `compozy agents list` for details",
		normalized,
		strings.Join(available, ", "),
	)
}

func writeAgentsListText(out io.Writer, catalog reusableagents.Catalog) error {
	if len(catalog.Agents) == 0 {
		if _, err := fmt.Fprintln(
			out,
			"no reusable agents found under .compozy/agents or ~/.compozy/agents",
		); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintf(out, "resolved reusable agents: %d\n", len(catalog.Agents)); err != nil {
			return err
		}
		for idx := range catalog.Agents {
			agent := &catalog.Agents[idx]
			if _, err := fmt.Fprintf(
				out,
				"\n%s\n  source: %s\n  description: %s\n  runtime: %s\n  mcp: %s\n",
				agent.Name,
				agent.Source.Scope,
				blankFallback(agent.Metadata.Description),
				formatRuntimeDefaults(agent.Runtime),
				formatMCPSummary(agent.MCP),
			); err != nil {
				return err
			}
		}
	}

	if len(catalog.Problems) == 0 {
		return nil
	}
	if _, err := fmt.Fprintf(out, "\ninvalid reusable agent definitions: %d\n", len(catalog.Problems)); err != nil {
		return err
	}
	for _, problem := range catalog.Problems {
		if _, err := fmt.Fprintf(
			out,
			"- %s (%s): %v\n",
			displayProblemName(problem),
			problem.Source.Scope,
			problem.Err,
		); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(
		out,
		"Run `compozy agents inspect <name>` to inspect one definition in detail.",
	); err != nil {
		return err
	}
	return nil
}

func writeInspectAgentText(out io.Writer, report inspectAgentReport) error {
	summary := strings.Join([]string{
		"Agent: " + report.Name,
		"Status: " + report.Status,
		"Source: " + string(report.Source.Scope),
		"Directory: " + blankFallback(report.Source.Dir),
		"Definition: " + blankFallback(report.Source.DefinitionPath),
		"MCP config: " + blankFallback(optionalExistingPath(report.Source.MCPConfigPath)),
		"Title: " + blankFallback(report.Metadata.Title),
		"Description: " + blankFallback(report.Metadata.Description),
		"Runtime defaults: " + formatRuntimeDefaults(report.Runtime),
	}, "\n")
	if _, err := fmt.Fprintln(out, summary); err != nil {
		return err
	}

	if report.MCP != nil && len(report.MCP.Servers) > 0 {
		if _, err := fmt.Fprintf(out, "MCP servers: %d\n", len(report.MCP.Servers)); err != nil {
			return err
		}
		for _, server := range report.MCP.Servers {
			if _, err := fmt.Fprintf(
				out,
				"- %s: command=%s args=%d env_keys=%s\n",
				server.Name,
				server.Command,
				len(server.Args),
				formatEnvKeys(server.Env),
			); err != nil {
				return err
			}
		}
	} else if _, err := fmt.Fprintln(out, "MCP servers: none"); err != nil {
		return err
	}

	if report.ValidationError == nil {
		_, err := fmt.Fprintln(out, "Validation: OK")
		return err
	}

	if _, err := fmt.Fprintf(out, "Validation: FAILED\nError: %v\n", report.ValidationError); err != nil {
		return err
	}
	_, err := fmt.Fprintf(
		out,
		"Next step: fix the agent definition and rerun `compozy agents inspect %s`\n",
		report.Name,
	)
	return err
}

func availableAgentNames(catalog reusableagents.Catalog) []string {
	names := make([]string, 0, len(catalog.Agents)+len(catalog.Problems))
	for idx := range catalog.Agents {
		names = append(names, catalog.Agents[idx].Name)
	}
	for _, problem := range catalog.Problems {
		name := strings.TrimSpace(problem.Name)
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func formatRuntimeDefaults(runtime reusableagents.RuntimeDefaults) string {
	parts := []string{
		formatRuntimePart("ide", runtime.IDE),
		formatRuntimePart("model", runtime.Model),
		formatRuntimePart("reasoning", runtime.ReasoningEffort),
		formatRuntimePart("access", runtime.AccessMode),
	}
	return strings.Join(parts, " ")
}

func formatRuntimePart(label, value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		trimmed = "-"
	}
	return label + "=" + trimmed
}

func formatMCPSummary(cfg *reusableagents.MCPConfig) string {
	if cfg == nil || len(cfg.Servers) == 0 {
		return "none"
	}
	names := make([]string, 0, len(cfg.Servers))
	for _, server := range cfg.Servers {
		names = append(names, server.Name)
	}
	return fmt.Sprintf("%d server(s): %s", len(cfg.Servers), strings.Join(names, ", "))
}

func formatEnvKeys(env map[string]string) string {
	if len(env) == 0 {
		return "-"
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return strings.Join(keys, ",")
}

func displayProblemName(problem reusableagents.Problem) string {
	name := strings.TrimSpace(problem.Name)
	if name != "" {
		return name
	}
	return filepath.Base(problem.Source.Dir)
}

func blankFallback(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func optionalExistingPath(path string) string {
	if strings.TrimSpace(path) == "" {
		return ""
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return ""
		}
		return path
	}
	return path
}
