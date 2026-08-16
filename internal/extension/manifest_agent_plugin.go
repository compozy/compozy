package extensionpkg

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	diagnosticspkg "github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/extension/agentplugin"
)

// ExtensionFormat identifies the manifest format that owns an extension instance.
type ExtensionFormat string

const (
	FormatCompozy     ExtensionFormat = "compozy"
	FormatAgentPlugin ExtensionFormat = "agent-plugin"
)

// SynthesizeAgentPluginManifest converts a validated portable package into a
// resource-only Compozy manifest without writing a second manifest to disk.
func SynthesizeAgentPluginManifest(pkg *agentplugin.Package, rootDir string) (*Manifest, error) {
	if pkg == nil {
		return nil, errors.New("extension: Agent Plugins package is required")
	}
	root := strings.TrimSpace(rootDir)
	if root == "" {
		return nil, errors.New("extension: Agent Plugins root directory is required")
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("extension: resolve Agent Plugins root %q: %w", root, err)
	}

	skills := make([]string, 0, len(pkg.Skills))
	for _, skill := range pkg.Skills {
		path := strings.TrimSpace(skill.SkillFile)
		if path == "" {
			path = filepath.Join(absRoot, "skills", skill.Name, "SKILL.md")
		}
		skills = append(skills, filepath.Clean(path))
	}
	sort.Strings(skills)

	servers := make(map[string]MCPServerConfig, len(pkg.Servers))
	for _, server := range pkg.Servers {
		name := strings.TrimSpace(server.Name)
		if name == "" {
			return nil, errors.New("extension: synthesized MCP server name is required")
		}
		config := MCPServerConfig{
			Command: strings.TrimSpace(server.Command),
			Args:    append([]string(nil), server.Args...),
			Env:     cloneStringMap(server.Env),
			Headers: cloneStringMap(server.Headers),
		}
		switch strings.TrimSpace(server.Transport) {
		case "stdio":
			config.Transport = "stdio"
		case "streamable-http":
			config.Transport = "http"
			config.URL = strings.TrimSpace(server.URL)
		default:
			return nil, fmt.Errorf("extension: unsupported synthesized MCP transport %q", server.Transport)
		}
		servers[name] = config
	}

	manifest := &Manifest{
		Format:            FormatAgentPlugin,
		IngestDiagnostics: AgentPluginDiagnostics(pkg.Name, pkg.Diagnostics),
		Name:              strings.TrimSpace(pkg.Name),
		Version:           strings.TrimSpace(pkg.Version),
		Description:       strings.TrimSpace(pkg.Description),
		Resources:         ResourcesConfig{Skills: skills, MCPServers: servers},
		Capabilities:      CapabilitiesConfig{},
		Permissions:       PermissionsConfig{},
		Subprocess:        SubprocessConfig{},
	}
	if err := manifest.Validate(); err != nil {
		return nil, err
	}
	return manifest, nil
}

// AgentPluginDiagnostics converts portable component skips into canonical recorded diagnostics.
func AgentPluginDiagnostics(name string, values []agentplugin.Diagnostic) []diagnosticcontract.DiagnosticItem {
	items := make([]diagnosticcontract.DiagnosticItem, 0, len(values))
	for _, value := range values {
		scope := strings.TrimSpace(value.Scope)
		message := strings.TrimSpace(value.Message)
		component := agentPluginDiagnosticComponent(scope)
		items = append(items, diagnosticspkg.NewItem(diagnosticspkg.ItemSpec{
			ID:            "extension/" + strings.TrimSpace(name) + "/agent-plugin/" + scope,
			Code:          diagnosticcontract.CodeExtensionAgentPluginComponentSkipped,
			Category:      diagnosticcontract.CategoryExtension,
			Title:         "Component skipped",
			Message:       agentPluginDiagnosticMessage(component, scope, message),
			Severity:      diagnosticcontract.SeverityWarn,
			DataFreshness: diagnosticcontract.FreshnessLive,
		}, diagnosticspkg.WithEvidence(map[string]any{"scope": scope, "component": component})))
	}
	sort.Slice(items, func(left, right int) bool {
		leftScope, _ := items[left].Evidence["scope"].(string)
		rightScope, _ := items[right].Evidence["scope"].(string)
		if leftScope != rightScope {
			return leftScope < rightScope
		}
		if items[left].Code != items[right].Code {
			return items[left].Code < items[right].Code
		}
		return items[left].Message < items[right].Message
	})
	return items
}

func agentPluginDiagnosticComponent(scope string) string {
	switch {
	case scope == "mcp" || strings.HasPrefix(scope, "mcp:"):
		return "mcp_server"
	case scope == "skills" || strings.HasPrefix(scope, "skill:"):
		return "skill"
	default:
		return "manifest"
	}
}

func agentPluginDiagnosticMessage(component, scope, message string) string {
	if component == "mcp_server" && strings.HasPrefix(scope, "mcp:") {
		return fmt.Sprintf("mcp server %q: %s", strings.TrimPrefix(scope, "mcp:"), message)
	}
	if component == "skill" && strings.HasPrefix(scope, "skill:") {
		return fmt.Sprintf("skill %q: %s", strings.TrimPrefix(scope, "skill:"), message)
	}
	return message
}
