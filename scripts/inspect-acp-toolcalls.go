package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/core/agent"
	"github.com/compozy/compozy/internal/core/model"
)

type inspectConfig struct {
	ides            []string
	model           string
	reasoningEffort string
	accessMode      string
	timeout         time.Duration
	workDir         string
	keepWorkDir     bool
	prompt          string
	webQuery        string
}

func main() {
	if err := run(context.Background(), os.Stdout, os.Stderr, os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "inspect-acp-toolcalls: %v\n", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, stdout io.Writer, stderr io.Writer, args []string) error {
	cfg, err := parseInspectConfig(args)
	if err != nil {
		return err
	}

	for _, ide := range cfg.ides {
		if err := inspectIDE(ctx, stdout, stderr, cfg, ide); err != nil {
			return err
		}
	}
	return nil
}

func parseInspectConfig(args []string) (inspectConfig, error) {
	fs := flag.NewFlagSet("inspect-acp-toolcalls", flag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var cfg inspectConfig
	var ideList string
	var timeout string
	fs.StringVar(
		&ideList,
		"ide",
		model.IDEClaude,
		"Comma-separated ACP runtimes to inspect (for example: claude,codex)",
	)
	fs.StringVar(&cfg.model, "model", "", "Optional model override")
	fs.StringVar(&cfg.reasoningEffort, "reasoning-effort", "medium", "Reasoning effort passed to the ACP driver")
	fs.StringVar(&cfg.accessMode, "access-mode", model.AccessModeFull, "Access mode to request from the ACP driver")
	fs.StringVar(&timeout, "timeout", "3m", "Maximum runtime per inspected driver")
	fs.StringVar(
		&cfg.workDir,
		"workdir",
		"",
		"Existing working directory to inspect instead of creating a temp workspace",
	)
	fs.BoolVar(&cfg.keepWorkDir, "keep-workdir", false, "Keep the generated temp workspace after the run")
	fs.StringVar(&cfg.prompt, "prompt", "", "Prompt to run instead of the default inspection scenario")
	fs.StringVar(
		&cfg.webQuery,
		"web-query",
		"Agent Client Protocol official documentation",
		"Topic used for the default web-search step",
	)

	if err := fs.Parse(args); err != nil {
		return inspectConfig{}, err
	}

	parsedTimeout, err := time.ParseDuration(timeout)
	if err != nil {
		return inspectConfig{}, fmt.Errorf("parse --timeout: %w", err)
	}
	cfg.timeout = parsedTimeout
	cfg.ides = splitIDEs(ideList)
	if len(cfg.ides) == 0 {
		return inspectConfig{}, fmt.Errorf("expected at least one --ide value")
	}
	return cfg, nil
}

func splitIDEs(value string) []string {
	parts := strings.Split(value, ",")
	ides := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			ides = append(ides, trimmed)
		}
	}
	return ides
}

func inspectIDE(ctx context.Context, stdout io.Writer, stderr io.Writer, cfg inspectConfig, ide string) error {
	workDir, cleanup, err := prepareInspectWorkspace(cfg)
	if err != nil {
		return err
	}
	defer cleanup()
	if cfg.keepWorkDir && strings.TrimSpace(cfg.workDir) == "" {
		fmt.Fprintf(stderr, "retaining temporary workspace for %s at %s\n", ide, workDir)
	}

	prompt := cfg.prompt
	if strings.TrimSpace(prompt) == "" {
		prompt = defaultInspectPrompt(cfg.webQuery)
	}

	runCtx, cancel := context.WithTimeout(ctx, cfg.timeout)
	defer cancel()

	client, err := agent.NewClient(runCtx, agent.ClientConfig{
		IDE:             ide,
		Model:           cfg.model,
		ReasoningEffort: cfg.reasoningEffort,
		AccessMode:      cfg.accessMode,
		ShutdownTimeout: 3 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("%s: create client: %w", ide, err)
	}
	defer func() {
		_ = client.Close()
	}()

	fmt.Fprintf(stdout, "=== %s (%s)\n", agent.DisplayName(ide), ide)
	fmt.Fprintf(stdout, "Working directory: %s\n", workDir)
	fmt.Fprintf(
		stdout,
		"Command preview: %s\n",
		agent.BuildShellCommandString(ide, cfg.model, nil, cfg.reasoningEffort, cfg.accessMode),
	)
	fmt.Fprintf(stdout, "Prompt:\n%s\n\n", prompt)

	session, err := client.CreateSession(runCtx, agent.SessionRequest{
		Prompt:     []byte(prompt),
		WorkingDir: workDir,
		Model:      cfg.model,
		ExtraEnv: map[string]string{
			"FORCE_COLOR":    "1",
			"CLICOLOR_FORCE": "1",
			"TERM":           "xterm-256color",
		},
	})
	if err != nil {
		return fmt.Errorf("%s: create session: %w", ide, err)
	}

	for update := range session.Updates() {
		renderUpdate(stdout, update)
	}
	if sessionErr := session.Err(); sessionErr != nil {
		return fmt.Errorf("%s: session failed: %w", ide, sessionErr)
	}

	fmt.Fprintln(stdout)
	return nil
}

func prepareInspectWorkspace(cfg inspectConfig) (string, func(), error) {
	if strings.TrimSpace(cfg.workDir) != "" {
		return cfg.workDir, func() {}, nil
	}

	workDir, err := os.MkdirTemp("", "compozy-acp-inspect-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp workspace: %w", err)
	}

	files := map[string]string{
		"README.md":       "# ACP Inspection Fixture\n\nThis workspace exists to exercise file-read and search tool calls.\n",
		"notes.txt":       "Initial notes.\n",
		"src/example.txt": "alpha\nbeta\ngamma\n",
	}
	for relativePath, body := range files {
		absPath := filepath.Join(workDir, relativePath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
			return "", nil, fmt.Errorf("create fixture directory %q: %w", absPath, err)
		}
		if err := os.WriteFile(absPath, []byte(body), 0o600); err != nil {
			return "", nil, fmt.Errorf("write fixture file %q: %w", absPath, err)
		}
	}

	cleanup := func() {
		if cfg.keepWorkDir {
			return
		}
		_ = os.RemoveAll(workDir)
	}
	return workDir, cleanup, nil
}

func defaultInspectPrompt(webQuery string) string {
	return strings.TrimSpace(fmt.Sprintf(`
Run this exact tool-call inspection sequence and stop when it is done:
1. Read README.md.
2. Append the line "ACP inspection marker" to notes.txt.
3. Use a web search tool to find the official source for %q and open the best result if the tool supports opening pages.
4. Reply with one short sentence that names the file you read, the file you edited, and the URL or query you used.

Prefer structured tools over shell commands whenever the driver offers them. Avoid unrelated work.
`, webQuery))
}

func renderUpdate(stdout io.Writer, update model.SessionUpdate) {
	fmt.Fprintf(
		stdout,
		"update kind=%q status=%q tool_call_id=%q tool_state=%q blocks=%d thought_blocks=%d\n",
		update.Kind,
		update.Status,
		update.ToolCallID,
		update.ToolCallState,
		len(update.Blocks),
		len(update.ThoughtBlocks),
	)

	renderBlocks(stdout, "block", update.Blocks)
	renderBlocks(stdout, "thought", update.ThoughtBlocks)

	if len(update.PlanEntries) > 0 {
		renderJSON(stdout, "plan", update.PlanEntries)
	}
	if len(update.AvailableCommands) > 0 {
		renderJSON(stdout, "commands", update.AvailableCommands)
	}
	if update.CurrentModeID != "" {
		fmt.Fprintf(stdout, "  current_mode=%q\n", update.CurrentModeID)
	}
	if update.Usage.Total() > 0 || update.Usage.CacheReads > 0 || update.Usage.CacheWrites > 0 {
		renderJSON(stdout, "usage", update.Usage)
	}
}

func renderBlocks(stdout io.Writer, prefix string, blocks []model.ContentBlock) {
	for index, block := range blocks {
		payload, err := json.MarshalIndent(block, "", "  ")
		if err != nil {
			fmt.Fprintf(stdout, "  %s[%d] marshal_error=%q\n", prefix, index, err)
			continue
		}
		fmt.Fprintf(stdout, "  %s[%d]=%s\n", prefix, index, string(payload))
	}
}

func renderJSON(stdout io.Writer, label string, value any) {
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "  %s_marshal_error=%q\n", label, err)
		return
	}
	fmt.Fprintf(stdout, "  %s=%s\n", label, string(payload))
}
