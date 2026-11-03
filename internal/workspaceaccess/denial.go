package workspaceaccess

// Denial names the decision-chain branch that refused an access request.
type Denial string

const (
	// DenialInvalid covers structural, lookup, and unknown-state failures.
	DenialInvalid Denial = "invalid"
	// DenialNotAgentSession covers non-operator actors that are not agent sessions.
	DenialNotAgentSession Denial = "not_agent_session"
	// DenialModeDenyAll is the deny-all permission mode, which never crosses.
	DenialModeDenyAll Denial = "mode_deny_all"
	// DenialConsentRequired is an approve-reads crossing without a recorded operator answer.
	DenialConsentRequired Denial = "consent_required"
	// DenialConsentRejected is an approve-reads crossing the operator rejected for the session.
	DenialConsentRejected Denial = "consent_rejected"
	// DenialNeedsGrant is a write to another profile or through an aggregate without the grant.
	DenialNeedsGrant Denial = "needs_grant"
)

// DenialMessage explains which axis refused the decision and what would allow it. It never
// names the target, so the message is identical whether or not a named target exists.
func DenialMessage(decision Decision) string {
	switch decision.Denial {
	case DenialModeDenyAll:
		return "this session's permissions.mode is deny-all, which never reaches another workspace, " +
			"profile, or All projects; ask the operator to set the agent's permissions.mode to " +
			"approve-all, or to approve-reads to be asked each time"
	case DenialConsentRequired:
		return "this session's permissions.mode is approve-reads, so reaching another workspace, " +
			"profile, or All projects needs the operator's approval in a prompt, and none was " +
			"answered here; retry from a native tool so the operator is asked, or ask the operator " +
			"to set the agent's permissions.mode to approve-all"
	case DenialConsentRejected:
		return "the operator rejected this access outside the session's workspace and profile; " +
			"ask the operator to set the agent's permissions.mode to approve-all if the session " +
			"needs that reach for reads; writes to another profile or All projects also need " +
			"workspace_access: unrestricted on the agent definition"
	case DenialNeedsGrant:
		return "permissions.mode reaches other profiles and All projects for reads only; changing " +
			"them needs workspace_access: unrestricted on the agent definition, which only the " +
			"operator can set"
	default:
		return DenialHint
	}
}
