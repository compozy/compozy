package extensionpkg

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/extension/agentplugin"
)

var (
	// ErrAgentPluginClientLayout reports a package nested under one client-specific manifest directory.
	ErrAgentPluginClientLayout = errors.New("extension: Agent Plugins client-specific layout")
	// ErrAgentPluginNotManifest reports a root plugin.json owned by another ecosystem.
	ErrAgentPluginNotManifest = errors.New("extension: plugin.json is not an Agent Plugins manifest")
	// ErrAgentPluginSchemaUnsupported reports a portable schema newer than this daemon supports.
	ErrAgentPluginSchemaUnsupported = errors.New("extension: Agent Plugins schema is unsupported")
	// ErrAgentPluginManifestInvalid reports fatal portable root-manifest issues.
	ErrAgentPluginManifestInvalid = errors.New("extension: agent plugin manifest invalid")
)

// AgentPluginClientLayoutError identifies one client-specific package layout.
type AgentPluginClientLayoutError struct {
	Root   string
	Layout string
}

func (e *AgentPluginClientLayoutError) Error() string {
	root := strings.TrimSpace(e.Root)
	layout := strings.TrimSpace(e.Layout)
	return fmt.Sprintf(
		"extension: %s is a %s plugin (%s/plugin.json), not an Agent Plugins package; "+
			"Compozy installs the standard layout (plugin.json at the package root) — "+
			"see https://agent-plugins.org/plugin-authors and the migrate-agent-plugin guide",
		root,
		agentPluginClientName(layout),
		layout,
	)
}

func (e *AgentPluginClientLayoutError) Unwrap() error { return ErrAgentPluginClientLayout }

// AgentPluginNotManifestError identifies an unrelated root plugin.json.
type AgentPluginNotManifestError struct {
	Root string
}

func (e *AgentPluginNotManifestError) Error() string {
	return fmt.Sprintf(
		"extension: %s has no recognized manifest; accepted roots are extension.toml, SKILL.md, "+
			"or an Agent Plugins plugin.json ($schema %s)",
		strings.TrimSpace(e.Root),
		agentplugin.PluginSchemaID,
	)
}

func (e *AgentPluginNotManifestError) Unwrap() error { return ErrAgentPluginNotManifest }

// AgentPluginSchemaUnsupportedError identifies the declared and supported portable schema versions.
type AgentPluginSchemaUnsupportedError struct {
	Root     string
	Declared string
}

func (e *AgentPluginSchemaUnsupportedError) Error() string {
	return fmt.Sprintf(
		"extension: %s declares Agent Plugins schema %q; this daemon supports 1.0.0",
		strings.TrimSpace(e.Root),
		strings.TrimSpace(e.Declared),
	)
}

func (e *AgentPluginSchemaUnsupportedError) Unwrap() error { return ErrAgentPluginSchemaUnsupported }

// AgentPluginManifestValidationError preserves every fatal portable manifest issue.
type AgentPluginManifestValidationError struct {
	Path   string
	Issues []agentplugin.Issue
}

func (e *AgentPluginManifestValidationError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return ErrAgentPluginManifestInvalid.Error()
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, strings.TrimSpace(issue.Message))
	}
	return ErrAgentPluginManifestInvalid.Error() + ": " + strings.Join(parts, "; ")
}

func (e *AgentPluginManifestValidationError) Unwrap() error { return ErrAgentPluginManifestInvalid }

func newAgentPluginManifestValidationError(path string, err *agentplugin.ManifestError) error {
	if err == nil {
		return ErrAgentPluginManifestInvalid
	}
	return &AgentPluginManifestValidationError{
		Path:   filepath.Clean(strings.TrimSpace(path)),
		Issues: append([]agentplugin.Issue(nil), err.Issues...),
	}
}

func agentPluginClientName(layout string) string {
	switch strings.TrimSpace(layout) {
	case ".claude-plugin":
		return "Claude Code"
	case ".codex-plugin":
		return "Codex"
	case ".cursor-plugin":
		return "Cursor"
	default:
		return "client-specific"
	}
}
