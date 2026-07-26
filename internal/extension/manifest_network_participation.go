package extensionpkg

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/network/participation"
)

// NetworkParticipationRequirement is the typed Live requirement declared on an extension manifest.
type NetworkParticipationRequirement struct {
	Required      bool     `toml:"required"                 json:"required"`
	Mode          string   `toml:"mode"                     json:"mode"`
	ChannelScopes []string `toml:"channel_scopes,omitempty" json:"channel_scopes,omitempty"`
}

// manifestNetworkRequirement keeps Manifest field lines under the golines budget.
type manifestNetworkRequirement = NetworkParticipationRequirement

func (r *NetworkParticipationRequirement) Normalize() *NetworkParticipationRequirement {
	if r == nil {
		return nil
	}
	scopes := make([]string, 0, len(r.ChannelScopes))
	seen := make(map[string]struct{}, len(r.ChannelScopes))
	for _, scope := range r.ChannelScopes {
		trimmed := strings.TrimSpace(scope)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		scopes = append(scopes, trimmed)
	}
	sort.Strings(scopes)
	return &NetworkParticipationRequirement{
		Required:      r.Required,
		Mode:          strings.TrimSpace(strings.ToLower(r.Mode)),
		ChannelScopes: scopes,
	}
}

func (r *NetworkParticipationRequirement) Validate(field string) error {
	if r == nil {
		return nil
	}
	normalized := r.Normalize()
	if !normalized.Required {
		if normalized.Mode != "" || len(normalized.ChannelScopes) > 0 {
			return fmt.Errorf("extension: %s mode and channel_scopes require required=true", field)
		}
		return nil
	}
	if participation.Mode(normalized.Mode) != participation.ModeLive {
		return fmt.Errorf("extension: %s.mode must be %q when required", field, participation.ModeLive)
	}
	return nil
}

// NetworkParticipationRequirementDigest returns the canonical digest for one requirement block.
func NetworkParticipationRequirementDigest(req *NetworkParticipationRequirement) (string, error) {
	normalized := req.Normalize()
	if normalized == nil || !normalized.Required {
		return "", nil
	}
	payload, err := json.Marshal(struct {
		Required      bool     `json:"required"`
		Mode          string   `json:"mode"`
		ChannelScopes []string `json:"channel_scopes"`
	}{
		Required:      true,
		Mode:          string(participation.ModeLive),
		ChannelScopes: normalized.ChannelScopes,
	})
	if err != nil {
		return "", fmt.Errorf("extension: marshal network participation requirement: %w", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
