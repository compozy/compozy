package marketplace

import (
	"encoding/json"
	"fmt"
	"strings"
)

const CatalogSource = "agh-catalog"

// EntryDetails is the typed public projection of one validated catalog payload.
type EntryDetails struct {
	Author    string
	Source    string
	MCP       *MCPEntryDetails
	Extension *ExtensionEntryDetails
	Skill     *SkillEntryDetails
}

// MCPEntryDetails contains the feed-locked MCP install template.
type MCPEntryDetails struct {
	Transport    string
	Command      string
	Args         []string
	URL          string
	OAuth        *MCPOAuthDetails
	Env          []MCPEnvFieldDetails
	DefaultScope string
}

type MCPOAuthDetails struct {
	IssuerURL        string
	AuthorizationURL string
	TokenURL         string
	ClientID         string
	Scopes           []string
}

type MCPEnvFieldDetails struct {
	Name     string
	Prompt   string
	Required bool
	Secret   bool
	Default  string
}

type ExtensionEntryDetails struct {
	InstallSlug  string
	ArtifactURL  string
	DigestSHA256 string
	Repository   string
}

type SkillEntryDetails struct {
	InstallSlug string
	DisplayName string
	Tags        []string
}

// ProjectEntry decodes one already-validated catalog row without duplicating feed schemas downstream.
func ProjectEntry(entry Entry) (EntryDetails, error) {
	switch entry.Kind {
	case KindMCP:
		return projectMCPEntry(entry)
	case KindExtension:
		return projectExtensionEntry(entry)
	case KindSkill:
		return projectSkillEntry(entry)
	default:
		return EntryDetails{}, fmt.Errorf("marketplace: project unsupported kind %q", entry.Kind)
	}
}

func projectMCPEntry(entry Entry) (EntryDetails, error) {
	var value mcpEntry
	if err := json.Unmarshal(entry.Payload, &value); err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: decode MCP entry %q: %w", entry.EntryID, err)
	}
	detail := &MCPEntryDetails{
		Transport:    strings.TrimSpace(value.Transport),
		Command:      strings.TrimSpace(value.Command),
		Args:         append([]string(nil), value.Args...),
		URL:          strings.TrimSpace(value.URL),
		DefaultScope: strings.TrimSpace(value.DefaultScope),
		Env:          make([]MCPEnvFieldDetails, 0, len(value.Env)),
	}
	for _, field := range value.Env {
		detail.Env = append(detail.Env, MCPEnvFieldDetails{
			Name: strings.TrimSpace(field.Name), Prompt: strings.TrimSpace(field.Prompt),
			Required: field.Required, Secret: field.Secret, Default: field.Default,
		})
	}
	if value.OAuth != nil {
		detail.OAuth = &MCPOAuthDetails{
			IssuerURL:        strings.TrimSpace(value.OAuth.IssuerURL),
			AuthorizationURL: strings.TrimSpace(value.OAuth.AuthorizationURL),
			TokenURL:         strings.TrimSpace(value.OAuth.TokenURL),
			ClientID:         strings.TrimSpace(value.OAuth.ClientID),
			Scopes:           append([]string(nil), value.OAuth.Scopes...),
		}
	}
	return EntryDetails{Source: CatalogSource, MCP: detail}, nil
}

func projectExtensionEntry(entry Entry) (EntryDetails, error) {
	var value extensionEntry
	if err := json.Unmarshal(entry.Payload, &value); err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: decode extension entry %q: %w", entry.EntryID, err)
	}
	return EntryDetails{
		Author: strings.TrimSpace(value.Author), Source: CatalogSource,
		Extension: &ExtensionEntryDetails{
			InstallSlug:  strings.TrimSpace(value.InstallSlug),
			ArtifactURL:  strings.TrimSpace(value.ArtifactURL),
			DigestSHA256: strings.ToLower(strings.TrimSpace(value.DigestSHA256)),
			Repository:   strings.TrimSpace(value.Repository),
		},
	}, nil
}

func projectSkillEntry(entry Entry) (EntryDetails, error) {
	var value skillEntry
	if err := json.Unmarshal(entry.Payload, &value); err != nil {
		return EntryDetails{}, fmt.Errorf("marketplace: decode skill entry %q: %w", entry.EntryID, err)
	}
	return EntryDetails{
		Author: strings.TrimSpace(value.Author), Source: CatalogSource,
		Skill: &SkillEntryDetails{
			InstallSlug: strings.TrimSpace(value.InstallSlug),
			DisplayName: strings.TrimSpace(value.DisplayName),
			Tags:        append([]string(nil), value.Tags...),
		},
	}, nil
}
