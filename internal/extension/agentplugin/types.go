// Package agentplugin reads and validates portable Agent Plugins packages.
package agentplugin

import (
	"fmt"
	"strings"
)

const (
	PluginSchemaID = "https://agent-plugins.org/schemas/1.0.0/plugin.schema.json"
	MCPSchemaID    = "https://agent-plugins.org/schemas/1.0.0/mcp.schema.json"

	PluginSchemaSHA256 = "0a4aad95ce337878ad38802ebf0daa3fde76abe3f65400c86bcbb1ec0b3ab883"
	MCPSchemaSHA256    = "6539175bfcdf43085855183e86da40ea94b166547a72b47ae9a0a390516d3acb"
)

const (
	schemaPrefix             = "https://agent-plugins.org/schemas/"
	fieldSchema              = "$schema"
	fieldName                = "name"
	fieldVersion             = "version"
	fieldDescription         = "description"
	fieldAuthor              = "author"
	fieldHomepage            = "homepage"
	fieldRepository          = "repository"
	fieldLicense             = "license"
	fieldKeywords            = "keywords"
	fieldEmail               = "email"
	fieldExtensions          = "extensions"
	fieldURL                 = "url"
	scopeManifest            = "manifest"
	scopeMCP                 = "mcp"
	scopeSkills              = "skills"
	fieldMCPServers          = "mcpServers"
	transportStdio           = "stdio"
	transportStreamableHTTP  = "streamable-http"
	transportSSE             = "sse"
	messageObjectWhenPresent = "must be an object when present"
)

// SchemaStatus classifies a root manifest without selecting a fallback.
type SchemaStatus int

const (
	SchemaSupported SchemaStatus = iota
	SchemaUnsupportedVersion
	SchemaUnrelated
)

// LoadOptions supplies the instance-specific data root used for expansion.
type LoadOptions struct {
	DataDir string
}

// Package is the validated, portable package projection.
type Package struct {
	Name        string
	Version     string
	Description string
	Author      *AuthorInfo
	Homepage    string
	Repository  string
	License     string
	Keywords    []string
	Skills      []SkillRef
	Servers     []ServerSpec
	Diagnostics []Diagnostic
}

// AuthorInfo mirrors the closed author object in the standard.
type AuthorInfo struct {
	Name  string
	Email string
	URL   string
}

// SkillRef identifies one immediate-child Agent Skill.
type SkillRef struct {
	Name      string
	Dir       string
	SkillFile string
}

// ServerSpec is a normalized stdio or Streamable HTTP MCP server.
type ServerSpec struct {
	Name      string
	Transport string
	Command   string
	CWD       string
	Args      []string
	Env       map[string]string
	URL       string
	Headers   map[string]string
}

// Diagnostic records one non-fatal component skip.
type Diagnostic struct {
	Scope   string
	Message string
}

// Issue identifies one fatal manifest validation problem.
type Issue struct {
	Path    string
	Message string
}

// ManifestError contains every deterministic fatal manifest issue.
type ManifestError struct {
	Issues []Issue
}

// Error renders all issues without discarding their field paths.
func (e *ManifestError) Error() string {
	if e == nil || len(e.Issues) == 0 {
		return "invalid Agent Plugins manifest"
	}
	parts := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		parts = append(parts, fmt.Sprintf("%s: %s", issue.Path, issue.Message))
	}
	return "invalid Agent Plugins manifest: " + strings.Join(parts, "; ")
}
