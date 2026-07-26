package core

import (
	"fmt"

	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/network"
	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"
	"github.com/gin-gonic/gin"
)

func sessionInfoFromAgentCaller(caller agentidentity.Caller) *session.Info {
	return &session.Info{
		ID:                   strings.TrimSpace(caller.Session.ID),
		Name:                 strings.TrimSpace(caller.Session.Name),
		AgentName:            strings.TrimSpace(caller.Session.AgentName),
		Provider:             strings.TrimSpace(caller.Session.Provider),
		Model:                strings.TrimSpace(caller.Session.Model),
		WorkspaceID:          strings.TrimSpace(caller.Session.WorkspaceID),
		Workspace:            strings.TrimSpace(caller.Session.WorkspacePath),
		NetworkParticipation: caller.Session.NetworkSpecSnapshot(),
		Type:                 caller.Session.Type,
		Lineage:              store.CloneSessionLineage(caller.Session.Lineage),
		State:                caller.Session.State,
		SoulSnapshotID:       strings.TrimSpace(caller.Session.SoulSnapshotID),
		SoulDigest:           strings.TrimSpace(caller.Session.SoulDigest),
		ParentSoulDigest:     strings.TrimSpace(caller.Session.ParentSoulDigest),
		CreatedAt:            caller.Session.CreatedAt,
		UpdatedAt:            caller.Session.UpdatedAt,
	}
}

func sortCoordinationChannels(channels []contract.CoordinationChannelPayload) {
	sort.SliceStable(channels, func(left, right int) bool {
		if channels[left].WorkspaceID != channels[right].WorkspaceID {
			return channels[left].WorkspaceID < channels[right].WorkspaceID
		}
		return channels[left].ID < channels[right].ID
	})
}

func parseBoolQuery(c *gin.Context, key string) (bool, error) {
	if c == nil {
		return false, nil
	}
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("query parameter %q must be a boolean: %w", key, err)
	}
	return parsed, nil
}

func parsePositiveIntQuery(c *gin.Context) (int, error) {
	const key = "limit"
	if c == nil {
		return 0, nil
	}
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return 0, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("query parameter %q must be a positive integer: %w", key, err)
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("query parameter %q must be a positive integer: %d", key, parsed)
	}
	return parsed, nil
}

func (h *BaseHandlers) nowUTC() time.Time {
	if h == nil || h.Now == nil {
		return time.Now().UTC()
	}
	return h.Now().UTC()
}

func envelopeTime(envelope network.Envelope) time.Time {
	if envelope.TS <= 0 {
		return time.Time{}
	}
	return time.Unix(envelope.TS, 0).UTC()
}

func zeroCoordinationMetadata(metadata contract.CoordinationMessageMetadataPayload) bool {
	return strings.TrimSpace(metadata.TaskID) == "" &&
		strings.TrimSpace(metadata.RunID) == "" &&
		strings.TrimSpace(metadata.WorkflowID) == "" &&
		strings.TrimSpace(metadata.ChannelID) == "" &&
		strings.TrimSpace(string(metadata.MessageKind)) == "" &&
		strings.TrimSpace(metadata.CorrelationID) == "" &&
		len(metadata.Ext) == 0
}

func optionalStringPtr(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}
