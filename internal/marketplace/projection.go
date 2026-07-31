package marketplace

import (
	"encoding/json"
	"fmt"
	"strings"
)

const CatalogSource = "compozy-catalog"

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
	Launch       MCPLaunchDetails
	Auth         *MCPAuthDetails
	Inputs       []MCPInputDetails
	DefaultScope string
}

type MCPLaunchDetails struct {
	Type    string
	Package string
	Version string
	Image   string
	Digest  string
	URL     string
	Args    []string
}

type MCPAuthDetails struct {
	Method       string
	Registration string
	Scopes       []string
}

type MCPInputDetails struct {
	ID       string
	Prompt   string
	Type     string
	Required bool
	Binding  MCPInputBindingDetails
	Default  json.RawMessage
}

type MCPInputBindingDetails struct {
	Type string
	Name string
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
		DefaultScope: strings.TrimSpace(value.DefaultScope),
		Launch: MCPLaunchDetails{
			Type: strings.TrimSpace(value.Launch.Type), Package: strings.TrimSpace(value.Launch.Package),
			Version: strings.TrimSpace(value.Launch.Version), Image: strings.TrimSpace(value.Launch.Image),
			Digest: strings.TrimSpace(value.Launch.Digest), URL: strings.TrimSpace(value.Launch.URL),
			Args: append([]string(nil), value.Launch.Args...),
		},
		Inputs: make([]MCPInputDetails, 0, len(value.Inputs)),
	}
	for _, input := range value.Inputs {
		detail.Inputs = append(detail.Inputs, MCPInputDetails{
			ID: strings.TrimSpace(input.ID), Prompt: strings.TrimSpace(input.Prompt), Type: input.Type,
			Required: input.Required,
			Binding:  MCPInputBindingDetails{Type: input.Binding.Type, Name: strings.TrimSpace(input.Binding.Name)},
			Default:  append(json.RawMessage(nil), input.Default...),
		})
	}
	if value.Auth != nil {
		detail.Auth = &MCPAuthDetails{
			Method: strings.TrimSpace(value.Auth.Method), Registration: strings.TrimSpace(value.Auth.Registration),
			Scopes: append([]string(nil), value.Auth.Scopes...),
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
