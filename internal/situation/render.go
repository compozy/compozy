package situation

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

const (
	promptContextOpen  = "<agh-situation-context>"
	promptContextClose = "</agh-situation-context>"
)

type renderedSection struct {
	name string
	raw  []byte
}

// RenderPrompt renders a deterministic JSON context block in the required
// `/agent/context` section order, omitting unavailable sections.
func RenderPrompt(payload *contract.AgentContextPayload) (string, error) {
	sections, err := renderSections(payload)
	if err != nil {
		return "", err
	}
	return renderPromptFromSections(sections)
}

func renderPromptFromSections(sections []renderedSection) (string, error) {
	if len(sections) == 0 {
		return "", nil
	}

	var builder strings.Builder
	builder.WriteString(promptContextOpen)
	builder.WriteString("\n{\n")
	for index, section := range sections {
		if index > 0 {
			builder.WriteString(",\n")
		}
		builder.WriteString("  ")
		name, err := json.Marshal(section.name)
		if err != nil {
			return "", fmt.Errorf("situation: marshal section name %q: %w", section.name, err)
		}
		builder.Write(name)
		builder.WriteString(": ")
		builder.Write(section.raw)
	}
	builder.WriteString("\n}\n")
	builder.WriteString(promptContextClose)
	return builder.String(), nil
}

func renderSections(payload *contract.AgentContextPayload) ([]renderedSection, error) {
	if payload == nil {
		return nil, nil
	}
	sections := make([]renderedSection, 0, 10)
	var err error

	sections, err = appendIdentitySections(sections, payload)
	if err != nil {
		return nil, err
	}
	sections, err = appendSoulSections(sections, payload)
	if err != nil {
		return nil, err
	}
	sections, err = appendRuntimeSections(sections, payload)
	if err != nil {
		return nil, err
	}
	return appendSupportSections(sections, payload)
}

func appendSoulSections(
	sections []renderedSection,
	payload *contract.AgentContextPayload,
) ([]renderedSection, error) {
	if hasSoul(payload.Soul) {
		return appendRenderedSection(sections, "soul", payload.Soul)
	}
	return sections, nil
}

func appendIdentitySections(
	sections []renderedSection,
	payload *contract.AgentContextPayload,
) ([]renderedSection, error) {
	var err error
	if hasSelf(payload.Self) {
		sections, err = appendRenderedSection(sections, "self", payload.Self)
		if err != nil {
			return nil, err
		}
	}
	if hasWorkspace(payload.Workspace) {
		sections, err = appendRenderedSection(sections, "workspace", payload.Workspace)
		if err != nil {
			return nil, err
		}
	}
	if hasSession(payload.Session) {
		sections, err = appendRenderedSection(sections, "session", payload.Session)
		if err != nil {
			return nil, err
		}
	}
	return sections, nil
}

func appendRuntimeSections(
	sections []renderedSection,
	payload *contract.AgentContextPayload,
) ([]renderedSection, error) {
	var err error
	if payload.Task.Available {
		sections, err = appendRenderedSection(sections, "task", payload.Task)
		if err != nil {
			return nil, err
		}
	}
	if payload.CoordinationChannel.Available {
		sections, err = appendRenderedSection(sections, "coordination_channel", payload.CoordinationChannel)
		if err != nil {
			return nil, err
		}
	}
	if hasListSection(payload.InboxSummary.Section) {
		sections, err = appendRenderedSection(sections, "inbox_summary", payload.InboxSummary)
		if err != nil {
			return nil, err
		}
	}
	if hasListSection(payload.PeerRoster.Section) {
		sections, err = appendRenderedSection(sections, "peer_roster", payload.PeerRoster)
		if err != nil {
			return nil, err
		}
	}
	return sections, nil
}

func appendSupportSections(
	sections []renderedSection,
	payload *contract.AgentContextPayload,
) ([]renderedSection, error) {
	var err error
	if hasListSection(payload.Capabilities.Section) {
		sections, err = appendRenderedSection(sections, "capabilities", payload.Capabilities)
		if err != nil {
			return nil, err
		}
	}
	if hasLimits(payload.Limits) {
		sections, err = appendRenderedSection(sections, "limits", payload.Limits)
		if err != nil {
			return nil, err
		}
	}
	if hasProvenance(payload.Provenance) {
		sections, err = appendRenderedSection(sections, "provenance", promptProvenancePayload{
			Source: strings.TrimSpace(payload.Provenance.Source),
		})
		if err != nil {
			return nil, err
		}
	}

	return sections, nil
}

func appendRenderedSection[T any](
	sections []renderedSection,
	name string,
	value T,
) ([]renderedSection, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("situation: marshal section %q: %w", name, err)
	}
	return append(sections, renderedSection{name: name, raw: raw}), nil
}

func hasSelf(payload contract.AgentIdentityPayload) bool {
	return strings.TrimSpace(payload.SessionID) != "" ||
		strings.TrimSpace(payload.AgentName) != "" ||
		strings.TrimSpace(payload.Provider) != "" ||
		strings.TrimSpace(payload.Model) != ""
}

func hasWorkspace(payload contract.AgentWorkspacePayload) bool {
	return strings.TrimSpace(payload.ID) != "" ||
		strings.TrimSpace(payload.Name) != "" ||
		strings.TrimSpace(payload.RootDir) != ""
}

func hasSession(payload contract.AgentSessionPayload) bool {
	return strings.TrimSpace(payload.ID) != ""
}

func hasSoul(payload contract.AgentSoulSectionPayload) bool {
	return payload.Enabled ||
		payload.Present ||
		payload.Active ||
		payload.Valid ||
		strings.TrimSpace(payload.SnapshotID) != "" ||
		strings.TrimSpace(payload.Digest) != ""
}

func hasListSection(payload contract.AgentContextSectionMetaPayload) bool {
	return payload.Limit > 0 || payload.Returned > 0 || payload.Truncated
}

func hasLimits(payload contract.AgentLimitsPayload) bool {
	return payload.MaxChildren > 0 ||
		payload.MaxSpawnDepth > 0 ||
		payload.MaxActiveTaskLeases > 0 ||
		payload.ContextSectionLimit > 0
}

func hasProvenance(payload contract.AgentContextProvenancePayload) bool {
	return strings.TrimSpace(payload.Source) != ""
}

type promptProvenancePayload struct {
	Source string `json:"source"`
}
