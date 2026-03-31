package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type resolvedEnvironment struct {
	cwd             string
	homeDir         string
	xdgConfigHome   string
	codeXHome       string
	claudeConfigDir string
}

type agentSpec struct {
	name        string
	displayName string
	projectDir  string
	globalDir   func(resolvedEnvironment) string
	detect      func(resolvedEnvironment) bool
}

func SupportedAgents(options ResolverOptions) ([]Agent, error) {
	env, err := resolveEnvironment(options)
	if err != nil {
		return nil, err
	}

	agents := make([]Agent, 0, len(agentSpecs))
	for _, spec := range agentSpecs {
		agents = append(agents, spec.agent(env))
	}
	return agents, nil
}

func DetectInstalledAgents(options ResolverOptions) ([]Agent, error) {
	env, err := resolveEnvironment(options)
	if err != nil {
		return nil, err
	}

	var detected []Agent
	for _, spec := range agentSpecs {
		if !spec.detect(env) {
			continue
		}
		detected = append(detected, spec.agent(env))
	}
	return detected, nil
}

func SelectAgents(all []Agent, names []string) ([]Agent, error) {
	if len(names) == 0 {
		return nil, fmt.Errorf("select setup agents: no agents requested")
	}

	index := make(map[string]Agent, len(all))
	aliases := make(map[string]string, len(agentAliases))
	for _, agent := range all {
		index[agent.Name] = agent
	}
	for alias, canonical := range agentAliases {
		aliases[alias] = canonical
	}

	selected := make([]Agent, 0, len(names))
	seen := make(map[string]struct{}, len(names))
	var invalid []string
	for _, name := range names {
		canonical := normalizeAgentName(name, aliases)
		agent, ok := index[canonical]
		if !ok {
			invalid = append(invalid, name)
			continue
		}
		if _, ok := seen[agent.Name]; ok {
			continue
		}
		seen[agent.Name] = struct{}{}
		selected = append(selected, agent)
	}
	if len(invalid) > 0 {
		slices.Sort(invalid)
		return nil, fmt.Errorf("select setup agents: invalid agent(s): %s", strings.Join(invalid, ", "))
	}

	slices.SortFunc(selected, func(left, right Agent) int {
		return strings.Compare(left.DisplayName, right.DisplayName)
	})
	return selected, nil
}

func normalizeAgentName(name string, aliases map[string]string) string {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}
	return normalized
}

func resolveEnvironment(options ResolverOptions) (resolvedEnvironment, error) {
	cwd := strings.TrimSpace(options.CWD)
	if cwd == "" {
		var err error
		cwd, err = os.Getwd()
		if err != nil {
			return resolvedEnvironment{}, fmt.Errorf("resolve setup environment cwd: %w", err)
		}
	}
	cwd = filepath.Clean(cwd)

	homeDir := strings.TrimSpace(options.HomeDir)
	if homeDir == "" {
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return resolvedEnvironment{}, fmt.Errorf("resolve setup environment home: %w", err)
		}
	}
	homeDir = filepath.Clean(homeDir)

	xdgConfigHome := strings.TrimSpace(options.XDGConfigHome)
	if xdgConfigHome == "" {
		xdgConfigHome = filepath.Join(homeDir, ".config")
	}

	codeXHome := strings.TrimSpace(options.CodeXHome)
	if codeXHome == "" {
		codeXHome = filepath.Join(homeDir, ".codex")
	}

	claudeConfigDir := strings.TrimSpace(options.ClaudeConfigDir)
	if claudeConfigDir == "" {
		claudeConfigDir = filepath.Join(homeDir, ".claude")
	}

	return resolvedEnvironment{
		cwd:             cwd,
		homeDir:         homeDir,
		xdgConfigHome:   filepath.Clean(xdgConfigHome),
		codeXHome:       filepath.Clean(codeXHome),
		claudeConfigDir: filepath.Clean(claudeConfigDir),
	}, nil
}

func (spec agentSpec) agent(env resolvedEnvironment) Agent {
	globalDir := spec.globalDir(env)
	return Agent{
		Name:           spec.name,
		DisplayName:    spec.displayName,
		ProjectRootDir: spec.projectDir,
		GlobalRootDir:  globalDir,
		Universal:      spec.projectDir == ".agents/skills",
		Detected:       spec.detect(env),
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

var agentAliases = map[string]string{
	"claude":      "claude-code",
	"claude-code": "claude-code",
}

var agentSpecs = []agentSpec{
	universalAgent("amp", "Amp", func(env resolvedEnvironment) string {
		return filepath.Join(env.xdgConfigHome, "agents", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.xdgConfigHome, "amp"))
	}),
	universalAgent("kimi-cli", "Kimi Code CLI", func(env resolvedEnvironment) string {
		return filepath.Join(env.xdgConfigHome, "agents", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".kimi"))
	}),
	universalAgent("replit", "Replit", func(env resolvedEnvironment) string {
		return filepath.Join(env.xdgConfigHome, "agents", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.cwd, ".replit"))
	}),
	universalAgent("universal", "Universal", func(env resolvedEnvironment) string {
		return filepath.Join(env.xdgConfigHome, "agents", "skills")
	}, func(resolvedEnvironment) bool {
		return false
	}),
	universalAgent("antigravity", "Antigravity", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".gemini", "antigravity", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".gemini", "antigravity"))
	}),
	specificAgent("augment", "Augment", ".augment/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".augment", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".augment"))
	}),
	specificAgent("claude-code", "Claude Code", ".claude/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.claudeConfigDir, "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(env.claudeConfigDir)
	}),
	specificAgent("openclaw", "OpenClaw", "skills", func(env resolvedEnvironment) string {
		if pathExists(filepath.Join(env.homeDir, ".openclaw")) {
			return filepath.Join(env.homeDir, ".openclaw", "skills")
		}
		if pathExists(filepath.Join(env.homeDir, ".clawdbot")) {
			return filepath.Join(env.homeDir, ".clawdbot", "skills")
		}
		if pathExists(filepath.Join(env.homeDir, ".moltbot")) {
			return filepath.Join(env.homeDir, ".moltbot", "skills")
		}
		return filepath.Join(env.homeDir, ".openclaw", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".openclaw")) ||
			pathExists(filepath.Join(env.homeDir, ".clawdbot")) ||
			pathExists(filepath.Join(env.homeDir, ".moltbot"))
	}),
	universalAgent("cline", "Cline", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".agents", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".cline"))
	}),
	specificAgent("codebuddy", "CodeBuddy", ".codebuddy/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".codebuddy", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.cwd, ".codebuddy")) || pathExists(filepath.Join(env.homeDir, ".codebuddy"))
	}),
	universalAgent("codex", "Codex", func(env resolvedEnvironment) string {
		return filepath.Join(env.codeXHome, "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(env.codeXHome) || pathExists("/etc/codex")
	}),
	specificAgent("command-code", "Command Code", ".commandcode/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".commandcode", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".commandcode"))
	}),
	specificAgent("continue", "Continue", ".continue/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".continue", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.cwd, ".continue")) || pathExists(filepath.Join(env.homeDir, ".continue"))
	}),
	specificAgent("cortex", "Cortex Code", ".cortex/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".snowflake", "cortex", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".snowflake", "cortex"))
	}),
	specificAgent("crush", "Crush", ".crush/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".config", "crush", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".config", "crush"))
	}),
	universalAgent("cursor", "Cursor", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".cursor", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".cursor"))
	}),
	universalAgent("deepagents", "Deep Agents", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".deepagents", "agent", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".deepagents"))
	}),
	specificAgent("droid", "Droid", ".factory/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".factory", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".factory"))
	}),
	universalAgent("firebender", "Firebender", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".firebender", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".firebender"))
	}),
	universalAgent("gemini-cli", "Gemini CLI", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".gemini", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".gemini"))
	}),
	universalAgent("github-copilot", "GitHub Copilot", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".copilot", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".copilot"))
	}),
	specificAgent("goose", "Goose", ".goose/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.xdgConfigHome, "goose", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.xdgConfigHome, "goose"))
	}),
	specificAgent("junie", "Junie", ".junie/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".junie", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".junie"))
	}),
	specificAgent("iflow-cli", "iFlow CLI", ".iflow/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".iflow", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".iflow"))
	}),
	specificAgent("kilo", "Kilo Code", ".kilocode/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".kilocode", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".kilocode"))
	}),
	specificAgent("kiro-cli", "Kiro CLI", ".kiro/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".kiro", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".kiro"))
	}),
	specificAgent("kode", "Kode", ".kode/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".kode", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".kode"))
	}),
	specificAgent("mcpjam", "MCPJam", ".mcpjam/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".mcpjam", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".mcpjam"))
	}),
	specificAgent("mistral-vibe", "Mistral Vibe", ".vibe/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".vibe", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".vibe"))
	}),
	specificAgent("mux", "Mux", ".mux/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".mux", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".mux"))
	}),
	universalAgent("opencode", "OpenCode", func(env resolvedEnvironment) string {
		return filepath.Join(env.xdgConfigHome, "opencode", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.xdgConfigHome, "opencode"))
	}),
	specificAgent("openhands", "OpenHands", ".openhands/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".openhands", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".openhands"))
	}),
	specificAgent("pi", "Pi", ".pi/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".pi", "agent", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".pi", "agent"))
	}),
	specificAgent("qoder", "Qoder", ".qoder/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".qoder", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".qoder"))
	}),
	specificAgent("qwen-code", "Qwen Code", ".qwen/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".qwen", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".qwen"))
	}),
	specificAgent("roo", "Roo Code", ".roo/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".roo", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".roo"))
	}),
	specificAgent("trae", "Trae", ".trae/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".trae", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".trae"))
	}),
	specificAgent("trae-cn", "Trae CN", ".trae/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".trae-cn", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".trae-cn"))
	}),
	universalAgent("warp", "Warp", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".agents", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".warp"))
	}),
	specificAgent("windsurf", "Windsurf", ".windsurf/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".codeium", "windsurf", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".codeium", "windsurf"))
	}),
	specificAgent("zencoder", "Zencoder", ".zencoder/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".zencoder", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".zencoder"))
	}),
	specificAgent("neovate", "Neovate", ".neovate/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".neovate", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".neovate"))
	}),
	specificAgent("pochi", "Pochi", ".pochi/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".pochi", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".pochi"))
	}),
	specificAgent("adal", "AdaL", ".adal/skills", func(env resolvedEnvironment) string {
		return filepath.Join(env.homeDir, ".adal", "skills")
	}, func(env resolvedEnvironment) bool {
		return pathExists(filepath.Join(env.homeDir, ".adal"))
	}),
}

func universalAgent(
	name string,
	displayName string,
	globalDir func(resolvedEnvironment) string,
	detect func(resolvedEnvironment) bool,
) agentSpec {
	return agentSpec{
		name:        name,
		displayName: displayName,
		projectDir:  ".agents/skills",
		globalDir:   globalDir,
		detect:      detect,
	}
}

func specificAgent(
	name string,
	displayName string,
	projectDir string,
	globalDir func(resolvedEnvironment) string,
	detect func(resolvedEnvironment) bool,
) agentSpec {
	return agentSpec{
		name:        name,
		displayName: displayName,
		projectDir:  projectDir,
		globalDir:   globalDir,
		detect:      detect,
	}
}
