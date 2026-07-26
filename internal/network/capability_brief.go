package network

import (
	"encoding/json"
	"fmt"
	"strings"

	sessionpkg "github.com/compozy/agh/internal/session"
)

const capabilityBriefExtKey = "agh.capabilities_brief"

type capabilityBrief struct {
	ID      string `json:"id"`
	Summary string `json:"summary"`
}

func projectCapabilityBriefView(
	capabilities []sessionpkg.NetworkPeerCapability,
) ([]string, ExtensionMap, error) {
	if len(capabilities) == 0 {
		return []string{}, nil, nil
	}

	ids := make([]string, 0, len(capabilities))
	brief := make([]capabilityBrief, 0, len(capabilities))
	for _, capability := range capabilities {
		id := strings.TrimSpace(capability.ID)
		if id == "" {
			continue
		}

		ids = append(ids, id)
		brief = append(brief, capabilityBrief{
			ID:      id,
			Summary: strings.TrimSpace(capability.Summary),
		})
	}
	if len(ids) == 0 {
		return []string{}, nil, nil
	}

	raw, err := json.Marshal(brief)
	if err != nil {
		return nil, nil, fmt.Errorf("network: marshal capability brief projection: %w", err)
	}

	return ids, ExtensionMap{
		capabilityBriefExtKey: raw,
	}, nil
}

func applyCapabilityBriefProjection(
	card *PeerCard,
	capabilities []sessionpkg.NetworkPeerCapability,
) error {
	if card == nil {
		return fmt.Errorf("%w: peer card is required", ErrInvalidField)
	}

	ids, briefExt, err := projectCapabilityBriefView(capabilities)
	if err != nil {
		return err
	}

	clonedExt := cloneExtensionMap(card.Ext)
	delete(clonedExt, capabilityBriefExtKey)
	if raw := briefExt[capabilityBriefExtKey]; len(raw) != 0 {
		if clonedExt == nil {
			clonedExt = make(ExtensionMap, 1)
		}
		clonedExt[capabilityBriefExtKey] = cloneRawMessage(raw)
	}
	if len(clonedExt) == 0 {
		clonedExt = nil
	}

	card.Capabilities = ids
	card.Ext = clonedExt
	return nil
}
