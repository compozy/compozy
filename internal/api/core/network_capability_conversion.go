package core

import (
	"encoding/json"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
)

func networkCapabilityBriefPayloads(
	card network.PeerCard,
	capabilityCatalog []session.NetworkPeerCapability,
) []contract.NetworkCapabilityBriefPayload {
	summaries := decodeCapabilityBriefSummaries(card.Ext)
	if len(capabilityCatalog) > 0 {
		orderedIDs := card.Capabilities
		if len(orderedIDs) == 0 {
			orderedIDs = make([]string, 0, len(capabilityCatalog))
			for _, capability := range capabilityCatalog {
				orderedIDs = append(orderedIDs, capability.ID)
			}
		}

		if summaries == nil {
			summaries = make(map[string]string, len(capabilityCatalog))
		}
		for _, capability := range capabilityCatalog {
			id := strings.TrimSpace(capability.ID)
			if id == "" {
				continue
			}
			summaries[id] = strings.TrimSpace(capability.Summary)
		}
		return capabilityBriefPayloadsFromIDs(orderedIDs, summaries)
	}

	return capabilityBriefPayloadsFromIDs(card.Capabilities, summaries)
}

func decodeCapabilityBriefSummaries(ext network.ExtensionMap) map[string]string {
	if len(ext) == 0 {
		return nil
	}

	raw, ok := ext[apiCapabilityBriefExtKey]
	if !ok || len(raw) == 0 {
		return nil
	}

	var payload []contract.NetworkCapabilityBriefPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	summaries := make(map[string]string, len(payload))
	for _, capability := range payload {
		id := strings.TrimSpace(capability.ID)
		if id == "" {
			continue
		}
		summaries[id] = strings.TrimSpace(capability.Summary)
	}
	if len(summaries) == 0 {
		return nil
	}
	return summaries
}

func capabilityBriefPayloadsFromIDs(
	capabilityIDs []string,
	summaries map[string]string,
) []contract.NetworkCapabilityBriefPayload {
	brief := make([]contract.NetworkCapabilityBriefPayload, 0, len(capabilityIDs))
	for _, capabilityID := range capabilityIDs {
		id := strings.TrimSpace(capabilityID)
		if id == "" {
			continue
		}

		brief = append(brief, contract.NetworkCapabilityBriefPayload{
			ID:      id,
			Summary: strings.TrimSpace(summaries[id]),
		})
	}
	if len(brief) == 0 {
		return []contract.NetworkCapabilityBriefPayload{}
	}
	return brief
}

func clonePeerCardExtWithoutCapabilityDiscovery(
	ext network.ExtensionMap,
) map[string]json.RawMessage {
	cloned := cloneRawMap(ext)
	if len(cloned) == 0 {
		return nil
	}

	delete(cloned, apiCapabilityBriefExtKey)
	delete(cloned, apiCapabilityCatalogExtKey)
	if len(cloned) == 0 {
		return nil
	}
	return cloned
}

func networkCapabilityCatalogPayload(
	peer network.PeerInfo,
) *contract.NetworkCapabilityCatalogPayload {
	if !peer.CapabilityCatalogKnown {
		return nil
	}

	payload := &contract.NetworkCapabilityCatalogPayload{
		Capabilities: make([]contract.NetworkCapabilityPayload, 0, len(peer.CapabilityCatalog)),
	}
	for _, capability := range peer.CapabilityCatalog {
		id := strings.TrimSpace(capability.ID)
		if id == "" {
			continue
		}

		payload.Capabilities = append(payload.Capabilities, contract.NetworkCapabilityPayload{
			ID:                id,
			Summary:           strings.TrimSpace(capability.Summary),
			Outcome:           strings.TrimSpace(capability.Outcome),
			Version:           strings.TrimSpace(capability.Version),
			Digest:            strings.TrimSpace(capability.Digest),
			ContextNeeded:     append([]string(nil), capability.ContextNeeded...),
			ArtifactsExpected: append([]string(nil), capability.ArtifactsExpected...),
			ExecutionOutline:  append([]string(nil), capability.ExecutionOutline...),
			Constraints:       append([]string(nil), capability.Constraints...),
			Examples:          append([]string(nil), capability.Examples...),
			Requirements:      append([]string(nil), capability.Requirements...),
		})
	}
	return payload
}
