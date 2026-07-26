package acpmock

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/kballard/go-shellquote"
)

// RegisterOptions describes one temporary fixture-backed AGENT.md registration.
type RegisterOptions struct {
	FixturePath     string
	FixtureAgent    string
	AgentName       string
	ProviderName    string
	DriverPath      string
	DiagnosticsPath string
}

// ProviderName is the auth-free local provider used by fixture-backed ACP mocks.
const ProviderName = "acpmock"

// Registration captures one temporary mock-agent definition written into AGH home.
type Registration struct {
	AgentName       string
	FixtureAgent    string
	FixturePath     string
	DriverPath      string
	DiagnosticsPath string
	AgentDefPath    string
	Command         string
	Provider        string
	Model           string
	Permissions     string
}

// Register writes one temporary fixture-backed AGENT.md file into the supplied AGH home.
func Register(homePaths aghconfig.HomePaths, opts RegisterOptions) (Registration, error) {
	if strings.TrimSpace(homePaths.AgentsDir) == "" {
		return Registration{}, errors.New("acpmock: home paths agents directory is required")
	}

	fixture, err := resolveRegistrationFixture(opts)
	if err != nil {
		return Registration{}, err
	}

	runtimeAgentNameInput := strings.TrimSpace(opts.AgentName)
	if runtimeAgentNameInput == "" {
		runtimeAgentNameInput = fixture.agentName
	}
	runtimeAgentName, err := sanitizeAgentPathSegment(runtimeAgentNameInput)
	if err != nil {
		return Registration{}, fmt.Errorf(
			"acpmock: validate runtime agent name %q: %w",
			runtimeAgentNameInput,
			err,
		)
	}

	driverPath, err := resolveDriverPath(opts.DriverPath)
	if err != nil {
		return Registration{}, fmt.Errorf("acpmock: resolve driver path: %w", err)
	}

	diagnosticsPath, err := resolveDiagnosticsPath(homePaths, runtimeAgentName, opts.DiagnosticsPath)
	if err != nil {
		return Registration{}, fmt.Errorf("acpmock: resolve diagnostics path for %q: %w", runtimeAgentName, err)
	}
	command := BuildCommand(driverPath, fixture.path, fixture.agentName, diagnosticsPath)
	providerName := registrationProviderName(opts.ProviderName, fixture.agent.Provider)

	agentDefPath := filepath.Join(homePaths.AgentsDir, runtimeAgentName, "AGENT.md")
	if err := os.MkdirAll(filepath.Dir(agentDefPath), 0o755); err != nil {
		return Registration{}, fmt.Errorf("acpmock: create agent directory %q: %w", filepath.Dir(agentDefPath), err)
	}

	content := renderAgentDef(runtimeAgentName, fixture.agent, command, providerName)
	if err := os.WriteFile(agentDefPath, []byte(content), 0o600); err != nil {
		return Registration{}, fmt.Errorf("acpmock: write agent definition %q: %w", agentDefPath, err)
	}

	loaded, err := aghconfig.LoadAgentDefFile(agentDefPath)
	if err != nil {
		return Registration{}, postWriteRegistrationError(
			agentDefPath,
			"validate written agent definition",
			err,
		)
	}
	cfg := aghconfig.DefaultWithHome(homePaths)
	if providerName == ProviderName {
		cfg.Providers[ProviderName] = ProviderConfig(driverPath)
	}
	if _, err := cfg.ResolveAgent(loaded); err != nil {
		return Registration{}, postWriteRegistrationError(
			agentDefPath,
			"resolve written agent definition",
			err,
		)
	}

	return Registration{
		AgentName:       runtimeAgentName,
		FixtureAgent:    fixture.agentName,
		FixturePath:     fixture.path,
		DriverPath:      driverPath,
		DiagnosticsPath: diagnosticsPath,
		AgentDefPath:    agentDefPath,
		Command:         command,
		Provider:        providerName,
		Model:           fixture.agent.Model,
		Permissions:     fixture.agent.Permissions,
	}, nil
}

// ProviderConfig returns the local auth-free provider config for ACP mock agents.
func ProviderConfig(command ...string) aghconfig.ProviderConfig {
	providerCommand := "acpmock-driver"
	if len(command) > 0 {
		if trimmed := strings.TrimSpace(command[0]); trimmed != "" {
			providerCommand = trimmed
		}
	}
	return aghconfig.ProviderConfig{
		Command:      providerCommand,
		DisplayName:  "ACP Mock",
		Harness:      aghconfig.ProviderHarnessACP,
		AuthMode:     aghconfig.ProviderAuthModeNone,
		NoneSecurity: aghconfig.ProviderNoneSecurityLocalTransport,
	}
}

func registrationProviderName(override string, fixtureProvider string) string {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		return trimmed
	}
	return strings.TrimSpace(fixtureProvider)
}

type registrationFixture struct {
	path      string
	agentName string
	agent     AgentFixture
}

func resolveRegistrationFixture(opts RegisterOptions) (registrationFixture, error) {
	fixturePath, err := aghconfig.ResolvePath(opts.FixturePath)
	if err != nil {
		return registrationFixture{}, fmt.Errorf("acpmock: resolve fixture path: %w", err)
	}
	fixture, err := LoadFixture(fixturePath)
	if err != nil {
		return registrationFixture{}, fmt.Errorf("acpmock: load fixture %q: %w", fixturePath, err)
	}
	agentName := strings.TrimSpace(opts.FixtureAgent)
	if agentName == "" {
		agentName = strings.TrimSpace(opts.AgentName)
	}
	if agentName == "" {
		return registrationFixture{}, errors.New("acpmock: fixture agent name is required")
	}
	agent, err := fixture.Agent(agentName)
	if err != nil {
		return registrationFixture{}, fmt.Errorf("acpmock: lookup fixture agent %q: %w", agentName, err)
	}
	return registrationFixture{path: fixturePath, agentName: agentName, agent: agent}, nil
}

// BuildCommand renders the test-only ACP driver command string stored in AGENT.md.
func BuildCommand(driverPath string, fixturePath string, fixtureAgent string, diagnosticsPath string) string {
	argv := []string{
		strings.TrimSpace(driverPath),
		"--fixture",
		strings.TrimSpace(fixturePath),
		"--agent",
		strings.TrimSpace(fixtureAgent),
	}
	if strings.TrimSpace(diagnosticsPath) != "" {
		argv = append(argv, "--diagnostics", strings.TrimSpace(diagnosticsPath))
	}
	return shellquote.Join(argv...)
}

func renderAgentDef(name string, agent AgentFixture, command string, providerName string) string {
	prompt := strings.TrimSpace(agent.Prompt)
	if prompt == "" {
		prompt = "You are " + name + "."
	}

	var builder strings.Builder
	builder.WriteString("---\n")
	builder.WriteString("name: " + strings.TrimSpace(name) + "\n")
	builder.WriteString("provider: " + strings.TrimSpace(providerName) + "\n")
	builder.WriteString("command: " + yamlSingleQuote(strings.TrimSpace(command)) + "\n")
	if model := strings.TrimSpace(agent.Model); model != "" {
		builder.WriteString("model: " + model + "\n")
	}
	if effort := strings.TrimSpace(agent.ReasoningEffort); effort != "" {
		builder.WriteString("reasoning_effort: " + effort + "\n")
	}
	if permissions := strings.TrimSpace(agent.Permissions); permissions != "" {
		builder.WriteString("permissions: " + permissions + "\n")
	}
	builder.WriteString("---\n\n")
	builder.WriteString(prompt)
	builder.WriteString("\n")
	return builder.String()
}

func yamlSingleQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func sanitizeAgentPathSegment(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", errors.New("acpmock: agent name is required")
	}
	if trimmed == "." || trimmed == ".." || strings.ContainsAny(trimmed, `/\`) {
		return "", fmt.Errorf("acpmock: invalid agent name %q", trimmed)
	}
	return trimmed, nil
}

func resolveDiagnosticsPath(homePaths aghconfig.HomePaths, name string, override string) (string, error) {
	if trimmed := strings.TrimSpace(override); trimmed != "" {
		resolved, err := aghconfig.ResolvePath(trimmed)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(resolved) == "" {
			return "", errors.New("acpmock: diagnostics path is required")
		}
		if err := os.MkdirAll(filepath.Dir(resolved), 0o755); err != nil {
			return "", fmt.Errorf("acpmock: create diagnostics directory %q: %w", filepath.Dir(resolved), err)
		}
		return resolved, nil
	}

	safeName, err := sanitizeAgentPathSegment(name)
	if err != nil {
		return "", fmt.Errorf("acpmock: validate diagnostics agent name: %w", err)
	}
	logsDir := strings.TrimSpace(homePaths.LogsDir)
	if logsDir == "" {
		return "", errors.New(
			"acpmock: home paths logs directory is required when diagnostics path override is not set",
		)
	}
	dir := filepath.Join(logsDir, "acpmock")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("acpmock: create diagnostics directory %q: %w", dir, err)
	}
	return filepath.Join(dir, safeName+".jsonl"), nil
}

func postWriteRegistrationError(agentDefPath string, action string, cause error) error {
	if cleanupErr := removeGeneratedAgentDef(agentDefPath); cleanupErr != nil {
		cause = errors.Join(cause, cleanupErr)
	}
	return fmt.Errorf("acpmock: %s %q: %w", action, agentDefPath, cause)
}

func removeGeneratedAgentDef(agentDefPath string) error {
	if err := os.Remove(agentDefPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("acpmock: remove generated agent definition %q: %w", agentDefPath, err)
	}
	return nil
}
