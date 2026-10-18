package task

import (
	"fmt"
	"strings"
	"time"
)

func (c ClaimCriteria) Normalize(defaultNow time.Time) (ClaimCriteria, error) {
	normalized := c
	normalized.RunID = strings.TrimSpace(normalized.RunID)
	normalized.Scope = normalized.Scope.Normalize()
	normalized.WorkspaceID = strings.TrimSpace(normalized.WorkspaceID)
	normalized.RunKind = normalized.RunKind.Normalize()
	if normalized.Scope == "" {
		if normalized.WorkspaceID != "" {
			normalized.Scope = ScopeWorkspace
		} else {
			normalized.Scope = ScopeGlobal
		}
	}
	normalized.ClaimerSessionID = strings.TrimSpace(normalized.ClaimerSessionID)
	if normalized.ClaimedBy != nil {
		claimedBy := *normalized.ClaimedBy
		claimedBy.Kind = claimedBy.Kind.Normalize()
		claimedBy.Ref = strings.TrimSpace(claimedBy.Ref)
		normalized.ClaimedBy = &claimedBy
	}
	if normalized.ClaimedBy == nil && normalized.ClaimerSessionID != "" {
		normalized.ClaimedBy = &ActorIdentity{
			Kind: ActorKindAgentSession,
			Ref:  normalized.ClaimerSessionID,
		}
	}
	normalized.AgentName = strings.TrimSpace(normalized.AgentName)
	normalized.RequiredCapabilities = normalizeCapabilityCriteria(normalized.RequiredCapabilities)
	if normalized.LeaseDuration == 0 {
		normalized.LeaseDuration = DefaultRunLeaseDuration
	}
	if normalized.Now.IsZero() {
		normalized.Now = defaultNow.UTC()
	} else {
		normalized.Now = normalized.Now.UTC()
	}
	if normalized.Soul != nil {
		soulProvenance := *normalized.Soul
		soulProvenance.SnapshotID = strings.TrimSpace(soulProvenance.SnapshotID)
		soulProvenance.Digest = strings.TrimSpace(soulProvenance.Digest)
		soulProvenance.AgentName = strings.TrimSpace(soulProvenance.AgentName)
		if soulProvenance.CapturedAt.IsZero() {
			soulProvenance.CapturedAt = normalized.Now
		} else {
			soulProvenance.CapturedAt = soulProvenance.CapturedAt.UTC()
		}
		normalized.Soul = &soulProvenance
	}
	if err := normalized.Validate("claim_criteria"); err != nil {
		return ClaimCriteria{}, err
	}
	return normalized, nil
}

// claimantRequiresSession reports whether the claimant identity is session-backed.
// Session-backed claims carry the executing session; daemon-owned claims bind one later.
func claimantRequiresSession(claimedBy *ActorIdentity) bool {
	return claimedBy == nil || claimedBy.Kind.Normalize() == ActorKindAgentSession
}

// Validate reports whether the claim criteria is safe to execute transactionally.
func (c ClaimCriteria) Validate(path string) error {
	if err := ValidateScopeBinding(c.Scope, c.WorkspaceID, path, "workspace_id"); err != nil {
		return err
	}
	if c.RunKind.Normalize() != RunKindUnknown {
		if err := c.RunKind.Validate(nestedPath(path, "run_kind")); err != nil {
			return err
		}
	}
	if strings.TrimSpace(c.ClaimerSessionID) == "" && claimantRequiresSession(c.ClaimedBy) {
		return fmt.Errorf(
			"%w: %s is required",
			ErrValidation,
			nestedPath(path, "claimer_session_id"),
		)
	}
	if c.ClaimedBy != nil {
		if err := c.ClaimedBy.Validate(nestedPath(path, "claimed_by")); err != nil {
			return err
		}
	}
	if err := ValidateCapabilityIDs(c.RequiredCapabilities, nestedPath(path, "required_capabilities")); err != nil {
		return err
	}
	if c.PriorityMin < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "priority_min"),
			c.PriorityMin,
		)
	}
	if c.WorkspaceActiveRunCap < 0 {
		return fmt.Errorf(
			"%w: %s must be zero or positive: %d",
			ErrValidation,
			nestedPath(path, "workspace_active_run_cap"),
			c.WorkspaceActiveRunCap,
		)
	}
	if err := validateLeaseDuration(c.LeaseDuration, nestedPath(path, "lease_duration")); err != nil {
		return err
	}
	if c.Soul != nil {
		if err := c.Soul.Validate(nestedPath(path, "soul")); err != nil {
			return err
		}
	}
	if c.Now.IsZero() {
		return fmt.Errorf("%w: %s is required", ErrValidation, nestedPath(path, "now"))
	}
	return nil
}
