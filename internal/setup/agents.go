package setup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type resolvedEnvironment struct {
	cwd             string
	homeDir         string
	xdgConfigHome   string
	codeXHome       string
	claudeConfigDir string
}

type envRoot uint8

const (
	envRootCWD envRoot = iota + 1
	envRootHome
	envRootXDGConfig
	envRootCodeX
	envRootClaudeConfig
	envRootAbsolute
)

type pathSpec struct {
	root     envRoot
	path     string
	absolute string
}

type pathChoice struct {
	detect pathSpec
	target pathSpec
}

type agentSpec struct {
	name             string
	displayName      string
	projectDir       string
	globalDir        pathSpec
	globalDirChoices []pathChoice
	detectPaths      []pathSpec
}

func SupportedAgents(options ResolverOptions) ([]Agent, error) {
	env, err := resolveEnvironment(options)
	if err != nil {
		return nil, err
	}

	agents := make([]Agent, 0, len(agentSpecs))
	for i := range agentSpecs {
		spec := &agentSpecs[i]
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
	for i := range agentSpecs {
		spec := &agentSpecs[i]
		if !spec.detected(env) {
			continue
		}
		detected = append(detected, spec.agent(env))
	}
	return detected, nil
}

func SelectAgents(all []Agent, names []string) ([]Agent, error) {
	return selectByName(all, names, selectByNameConfig[Agent]{
		subject:      "setup agents",
		emptyLabel:   "agents",
		invalidLabel: "agent(s)",
		getName:      func(agent Agent) string { return agent.Name },
		normalize:    normalizeAgentName,
		less:         func(left, right Agent) int { return strings.Compare(left.DisplayName, right.DisplayName) },
	})
}

func normalizeAgentName(name string) string {
	normalized := strings.TrimSpace(strings.ToLower(name))
	if canonical, ok := agentAliases[normalized]; ok {
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
	return Agent{
		Name:           spec.name,
		DisplayName:    spec.displayName,
		ProjectRootDir: spec.projectDir,
		GlobalRootDir:  spec.resolveGlobalDir(env),
		Universal:      spec.projectDir == ".agents/skills",
		Detected:       spec.detected(env),
	}
}

func (spec agentSpec) resolveGlobalDir(env resolvedEnvironment) string {
	if len(spec.globalDirChoices) == 0 {
		return spec.globalDir.resolve(env)
	}

	for _, choice := range spec.globalDirChoices {
		if pathExists(choice.detect.resolve(env)) {
			return choice.target.resolve(env)
		}
	}

	return spec.globalDirChoices[0].target.resolve(env)
}

func (spec agentSpec) detected(env resolvedEnvironment) bool {
	for _, detectPath := range spec.detectPaths {
		if pathExists(detectPath.resolve(env)) {
			return true
		}
	}
	return false
}

func (spec pathSpec) resolve(env resolvedEnvironment) string {
	if spec.absolute != "" {
		return filepath.Clean(spec.absolute)
	}

	var base string
	switch spec.root {
	case envRootCWD:
		base = env.cwd
	case envRootHome:
		base = env.homeDir
	case envRootXDGConfig:
		base = env.xdgConfigHome
	case envRootCodeX:
		base = env.codeXHome
	case envRootClaudeConfig:
		base = env.claudeConfigDir
	default:
		return ""
	}

	return filepath.Clean(filepath.Join(base, spec.path))
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func cwdPath(path string) pathSpec {
	return pathSpec{root: envRootCWD, path: path}
}

func homePath(path string) pathSpec {
	return pathSpec{root: envRootHome, path: path}
}

func xdgPath(path string) pathSpec {
	return pathSpec{root: envRootXDGConfig, path: path}
}

func codexPath(path string) pathSpec {
	return pathSpec{root: envRootCodeX, path: path}
}

func claudeConfigPath(path string) pathSpec {
	return pathSpec{root: envRootClaudeConfig, path: path}
}

func absolutePath(path string) pathSpec {
	return pathSpec{root: envRootAbsolute, absolute: path}
}

var agentAliases = map[string]string{
	"claude":      "claude-code",
	"claude-code": "claude-code",
}

var agentSpecs = []agentSpec{
	universalAgent("amp", "Amp", xdgPath("agents/skills"), xdgPath("amp")),
	universalAgent("kimi-cli", "Kimi Code CLI", xdgPath("agents/skills"), homePath(".kimi")),
	universalAgent("replit", "Replit", xdgPath("agents/skills"), cwdPath(".replit")),
	universalAgent("universal", "Universal", xdgPath("agents/skills")),
	universalAgent(
		"antigravity",
		"Antigravity",
		homePath(".gemini/antigravity/skills"),
		homePath(".gemini/antigravity"),
	),
	specificAgent("augment", "Augment", ".augment/skills", homePath(".augment/skills"), homePath(".augment")),
	specificAgent("claude-code", "Claude Code", ".claude/skills", claudeConfigPath("skills"), claudeConfigPath("")),
	choiceAgent(
		"openclaw",
		"OpenClaw",
		"skills",
		[]pathChoice{
			{detect: homePath(".openclaw"), target: homePath(".openclaw/skills")},
			{detect: homePath(".clawdbot"), target: homePath(".clawdbot/skills")},
			{detect: homePath(".moltbot"), target: homePath(".moltbot/skills")},
		},
		homePath(".openclaw"),
		homePath(".clawdbot"),
		homePath(".moltbot"),
	),
	universalAgent("cline", "Cline", homePath(".agents/skills"), homePath(".cline")),
	specificAgent(
		"codebuddy",
		"CodeBuddy",
		".codebuddy/skills",
		homePath(".codebuddy/skills"),
		cwdPath(".codebuddy"),
		homePath(".codebuddy"),
	),
	universalAgent("codex", "Codex", codexPath("skills"), codexPath(""), absolutePath("/etc/codex")),
	specificAgent(
		"command-code",
		"Command Code",
		".commandcode/skills",
		homePath(".commandcode/skills"),
		homePath(".commandcode"),
	),
	specificAgent(
		"continue",
		"Continue",
		".continue/skills",
		homePath(".continue/skills"),
		cwdPath(".continue"),
		homePath(".continue"),
	),
	specificAgent(
		"cortex",
		"Cortex Code",
		".cortex/skills",
		homePath(".snowflake/cortex/skills"),
		homePath(".snowflake/cortex"),
	),
	specificAgent("crush", "Crush", ".crush/skills", xdgPath("crush/skills"), xdgPath("crush")),
	universalAgent("cursor", "Cursor", homePath(".cursor/skills"), homePath(".cursor")),
	universalAgent("deepagents", "Deep Agents", homePath(".deepagents/agent/skills"), homePath(".deepagents")),
	specificAgent("droid", "Droid", ".factory/skills", homePath(".factory/skills"), homePath(".factory")),
	universalAgent("firebender", "Firebender", homePath(".firebender/skills"), homePath(".firebender")),
	universalAgent("gemini-cli", "Gemini CLI", homePath(".gemini/skills"), homePath(".gemini")),
	universalAgent("github-copilot", "GitHub Copilot", homePath(".copilot/skills"), homePath(".copilot")),
	specificAgent("goose", "Goose", ".goose/skills", xdgPath("goose/skills"), xdgPath("goose")),
	specificAgent("junie", "Junie", ".junie/skills", homePath(".junie/skills"), homePath(".junie")),
	specificAgent("iflow-cli", "iFlow CLI", ".iflow/skills", homePath(".iflow/skills"), homePath(".iflow")),
	specificAgent("kilo", "Kilo Code", ".kilocode/skills", homePath(".kilocode/skills"), homePath(".kilocode")),
	specificAgent("kiro-cli", "Kiro CLI", ".kiro/skills", homePath(".kiro/skills"), homePath(".kiro")),
	specificAgent("kode", "Kode", ".kode/skills", homePath(".kode/skills"), homePath(".kode")),
	specificAgent("mcpjam", "MCPJam", ".mcpjam/skills", homePath(".mcpjam/skills"), homePath(".mcpjam")),
	specificAgent(
		"mistral-vibe",
		"Mistral Vibe",
		".vibe/skills",
		homePath(".vibe/skills"),
		homePath(".vibe"),
	),
	specificAgent("mux", "Mux", ".mux/skills", homePath(".mux/skills"), homePath(".mux")),
	universalAgent("opencode", "OpenCode", xdgPath("opencode/skills"), xdgPath("opencode")),
	specificAgent("openhands", "OpenHands", ".openhands/skills", homePath(".openhands/skills"), homePath(".openhands")),
	specificAgent("pi", "Pi", ".pi/skills", homePath(".pi/agent/skills"), homePath(".pi/agent")),
	specificAgent("qoder", "Qoder", ".qoder/skills", homePath(".qoder/skills"), homePath(".qoder")),
	specificAgent("qwen-code", "Qwen Code", ".qwen/skills", homePath(".qwen/skills"), homePath(".qwen")),
	specificAgent("roo", "Roo Code", ".roo/skills", homePath(".roo/skills"), homePath(".roo")),
	specificAgent("trae", "Trae", ".trae/skills", homePath(".trae/skills"), homePath(".trae")),
	specificAgent("trae-cn", "Trae CN", ".trae/skills", homePath(".trae-cn/skills"), homePath(".trae-cn")),
	universalAgent("warp", "Warp", homePath(".agents/skills"), homePath(".warp")),
	specificAgent(
		"windsurf",
		"Windsurf",
		".windsurf/skills",
		homePath(".codeium/windsurf/skills"),
		homePath(".codeium/windsurf"),
	),
	specificAgent("zencoder", "Zencoder", ".zencoder/skills", homePath(".zencoder/skills"), homePath(".zencoder")),
	specificAgent("neovate", "Neovate", ".neovate/skills", homePath(".neovate/skills"), homePath(".neovate")),
	specificAgent("pochi", "Pochi", ".pochi/skills", homePath(".pochi/skills"), homePath(".pochi")),
	specificAgent("adal", "AdaL", ".adal/skills", homePath(".adal/skills"), homePath(".adal")),
}

func universalAgent(name string, displayName string, globalDir pathSpec, detectPaths ...pathSpec) agentSpec {
	return agentSpec{
		name:        name,
		displayName: displayName,
		projectDir:  ".agents/skills",
		globalDir:   globalDir,
		detectPaths: detectPaths,
	}
}

func specificAgent(
	name string,
	displayName string,
	projectDir string,
	globalDir pathSpec,
	detectPaths ...pathSpec,
) agentSpec {
	return agentSpec{
		name:        name,
		displayName: displayName,
		projectDir:  projectDir,
		globalDir:   globalDir,
		detectPaths: detectPaths,
	}
}

func choiceAgent(
	name string,
	displayName string,
	projectDir string,
	globalDirChoices []pathChoice,
	detectPaths ...pathSpec,
) agentSpec {
	return agentSpec{
		name:             name,
		displayName:      displayName,
		projectDir:       projectDir,
		globalDirChoices: globalDirChoices,
		detectPaths:      detectPaths,
	}
}
