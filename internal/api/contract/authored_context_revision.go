package contract

import (
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/diagnostics"
	heartbeatpkg "github.com/compozy/agh/internal/heartbeat"
	soulpkg "github.com/compozy/agh/internal/soul"
)

// AgentSoulRevisionPayloadFromDomain converts a Soul authoring revision into the public DTO.
func AgentSoulRevisionPayloadFromDomain(revision soulpkg.Revision) (AgentSoulRevisionPayload, error) {
	normalized := revision.Normalize()
	action := AgentSoulRevisionAction(normalized.Action)
	if !validAgentSoulRevisionAction(action) {
		return AgentSoulRevisionPayload{}, fmt.Errorf("%w: soul revision action %q",
			ErrInvalidAuthoredContextEnum,
			action,
		)
	}
	diagnostics, err := decodeSoulRevisionDiagnostics(normalized.DiagnosticsJSON)
	if err != nil {
		return AgentSoulRevisionPayload{}, err
	}
	origin := authoredActorPtr(normalized.OriginKind, normalized.OriginRef)
	return AgentSoulRevisionPayload{
		ID:             normalized.ID,
		AgentName:      normalized.AgentName,
		SourcePath:     normalized.SourcePath,
		Action:         action,
		PreviousDigest: normalized.PreviousDigest,
		NewDigest:      normalized.NewDigest,
		Diagnostics:    diagnostics,
		Actor: AuthoredContextActorPayload{
			Kind: normalized.ActorKind,
			Ref:  normalized.ActorID,
		},
		Origin:    origin,
		CreatedAt: normalized.CreatedAt.UTC(),
	}, nil
}

// HeartbeatRevisionPayloadFromDomain converts a Heartbeat authoring revision into the public DTO.
func HeartbeatRevisionPayloadFromDomain(revision heartbeatpkg.Revision) (HeartbeatRevisionPayload, error) {
	normalized := revision.Normalize()
	operation := HeartbeatRevisionOperation(normalized.Operation)
	if !validHeartbeatRevisionOperation(operation) {
		return HeartbeatRevisionPayload{}, fmt.Errorf("%w: heartbeat revision operation %q",
			ErrInvalidAuthoredContextEnum,
			operation,
		)
	}
	actorKind := HeartbeatActorKind(normalized.ActorKind)
	if !validHeartbeatActorKind(actorKind) {
		return HeartbeatRevisionPayload{}, fmt.Errorf("%w: heartbeat actor kind %q",
			ErrInvalidAuthoredContextEnum,
			actorKind,
		)
	}
	return HeartbeatRevisionPayload{
		ID:             normalized.ID,
		AgentName:      normalized.AgentName,
		SourcePath:     normalized.SourcePath,
		Operation:      operation,
		PreviousDigest: normalized.PreviousDigest,
		NewDigest:      normalized.NewDigest,
		NewSnapshotID:  normalized.NewSnapshotID,
		Actor: HeartbeatActorPayload{
			Kind: actorKind,
			Ref:  normalized.ActorID,
		},
		CreatedAt: normalized.CreatedAt.UTC(),
	}, nil
}

func containsUnsafeAuthoredContextJSON(data []byte) bool {
	return containsUnsafePublicContractJSON(data)
}

func isUnsafeAuthoredContextString(value string) bool {
	return isUnsafePublicContractString(value)
}

func sanitizeAuthoredContextLastError(value string) string {
	redacted := diagnostics.RedactAndBound(value, maxAuthoredContextLastErrorBytes)
	if isUnsafeAuthoredContextString(redacted) {
		return "[REDACTED]"
	}
	return redacted
}

func validationStatus(present bool, active bool, valid bool) AuthoredValidationStatus {
	switch {
	case !present:
		return AuthoredValidationMissing
	case !valid:
		return AuthoredValidationInvalid
	case !active:
		return AuthoredValidationInactive
	default:
		return AuthoredValidationValid
	}
}

func agentSoulFrontmatterPayload(front soulpkg.Frontmatter) AgentSoulFrontmatterPayload {
	return AgentSoulFrontmatterPayload{
		Version:       strings.TrimSpace(front.Version),
		Role:          strings.TrimSpace(front.Role),
		Tone:          normalizeAuthoredStrings(front.Tone),
		Principles:    normalizeAuthoredStrings(front.Principles),
		Constraints:   normalizeAuthoredStrings(front.Constraints),
		Collaboration: normalizeAuthoredStrings(front.Collaboration),
		MemoryPolicy:  normalizeAuthoredStrings(front.MemoryPolicy),
		Tags:          normalizeAuthoredStrings(front.Tags),
	}
}
