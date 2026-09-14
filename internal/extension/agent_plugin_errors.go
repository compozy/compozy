package extensionpkg

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/compozy/internal/extension/agentplugin"
)

var (
	// ErrAgentPluginNotManifest reports a root plugin.json owned by another ecosystem.
	ErrAgentPluginNotManifest = errors.New("extension: plugin.json is not an Agent Plugins manifest")
	// ErrAgentPluginManifestInvalid reports fatal portable root-manifest issues.
	ErrAgentPluginManifestInvalid = errors.New("extension: agent plugin manifest invalid")
)

// AgentPluginNotManifestError identifies an unrelated root plugin.json.
type AgentPluginNotManifestError struct {
	Checked []string
	Root    string
}

func (e *AgentPluginNotManifestError) Error() string {
	if len(e.Checked) > 0 {
		return fmt.Sprintf(
			"extension: %s has no recognized plugin manifest; checked %s",
			e.Root,
			strings.Join(e.Checked, ", "),
		)
	}
	return fmt.Sprintf(
		"extension: %s has no recognized manifest; accepted roots are extension.toml, SKILL.md, "+
			"or an Agent Plugins plugin.json ($schema %s)",
		strings.TrimSpace(e.Root),
		agentplugin.PluginSchemaID,
	)
}

func (e *AgentPluginNotManifestError) Unwrap() error { return ErrAgentPluginNotManifest }

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
