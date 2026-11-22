package session

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subagentid"
)

const subagentRoleGeneral = "general"

func normalizeIsolation(req *SubagentRequest) error {
	if req.Isolation == "" {
		req.Isolation = subagentIsolationShared
	}
	if req.Isolation != subagentIsolationShared && req.Isolation != SubagentIsolationWorktree {
		return invalidSubagent("isolation must be shared or worktree.")
	}
	req.BaseRef = strings.TrimSpace(req.BaseRef)
	if req.BaseRef != "" && req.Isolation != SubagentIsolationWorktree {
		return invalidSubagent(`base_ref requires isolation "worktree".`)
	}
	return nil
}

func validateHandoff(req *SubagentRequest) error {
	req.PriorSubagentID = strings.TrimSpace(req.PriorSubagentID)
	if utf8.RuneCountInString(req.PriorSubagentID) > 64 {
		return invalidSubagent("prior_subagent_id exceeds 64 characters.")
	}
	if req.Handoff != nil {
		req.Handoff.Objections = strings.TrimSpace(req.Handoff.Objections)
		req.Handoff.Findings = strings.TrimSpace(req.Handoff.Findings)
		if utf8.RuneCountInString(req.Handoff.Objections) > 40000 {
			return invalidSubagent("handoff.objections exceeds 40000 characters.")
		}
		if utf8.RuneCountInString(req.Handoff.Findings) > 40000 {
			return invalidSubagent("handoff.findings exceeds 40000 characters.")
		}
	}
	return nil
}

func normalizeSubagentRequest(req SubagentRequest) (SubagentRequest, error) {
	if err := normalizeIsolation(&req); err != nil {
		return req, err
	}
	if strings.TrimSpace(req.Task) == "" {
		return req, invalidSubagent("task is required.")
	}
	if utf8.RuneCountInString(req.Task) > 120000 {
		return req, invalidSubagent("task exceeds 120000 characters.")
	}
	if err := validateHandoff(&req); err != nil {
		return req, err
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = strings.TrimSpace(strings.SplitN(strings.TrimSpace(req.Task), "\n", 2)[0])
	}
	if utf8.RuneCountInString(req.Title) > 512 {
		return req, invalidSubagent("title exceeds 512 characters.")
	}
	req.Task = formatTaskWithHandoff(req.Task, req.PriorSubagentID, req.Handoff)
	if utf8.RuneCountInString(req.Task) > 120000 {
		return req, invalidSubagent("task exceeds 120000 characters.")
	}
	if req.Role == "" {
		req.Role = subagentRoleGeneral
	}
	switch req.Role {
	case subagentRoleGeneral, "implementation", "research", "review", "design", "test":
	default:
		return req, invalidSubagent("invalid role.")
	}
	if req.Mode == "" {
		req.Mode = SubagentModeAsync
	}
	if req.Mode != SubagentModeAsync && req.Mode != SubagentModeWait {
		return req, invalidSubagent("mode must be async or wait.")
	}
	if req.Mode == SubagentModeAsync {
		req.Timeout = 0
	} else {
		if req.Timeout == 0 {
			req.Timeout = 10 * time.Minute
		}
		req.Timeout = max(time.Second, min(time.Hour, req.Timeout))
	}
	req.IdempotencyKey = strings.TrimSpace(req.IdempotencyKey)
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = req.Caller.ToolCallID
	}
	if req.IdempotencyKey == "" || utf8.RuneCountInString(req.IdempotencyKey) > 256 {
		return req, invalidSubagent("idempotency_key must contain 1 to 256 characters.")
	}
	return req, nil
}

func invalidSubagent(message string) error {
	return &SubagentError{Code: "invalid_request", Message: message, Err: ErrSubagentInvalidRequest}
}
func subagentChildSessionID(parent, subagent string) string {
	return "sess-" + strings.TrimPrefix(subagentID(parent, subagent), "sub-")
}

func subagentID(parent, key string) string { return subagentid.Derive(parent, key) }

func subagentFingerprint(req SubagentRequest) (string, error) {
	// Invocation identity is outside the request semantics, allowing retries from another tool call.
	req.Caller = SubagentCaller{}
	// A shared delegation hashes in its pre-isolation shape, so retries of delegations admitted before the
	// upgrade replay their original result; only worktree requests carry the isolation fields.
	if req.Isolation == subagentIsolationShared {
		req.Isolation = ""
	}
	payload, err := json.Marshal(struct {
		SubagentRequest
		Timeout int64
	}{SubagentRequest: req, Timeout: int64(req.Timeout)}, json.Deterministic(true), json.FormatNilSliceAsNull(true))
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:]), nil
}
func formatTaskWithHandoff(task, priorID string, handoff *SubagentHandoff) string {
	if priorID == "" && handoff == nil {
		return task
	}
	var sb strings.Builder
	sb.WriteString("[Corrective / follow-up round")
	if priorID != "" {
		sb.WriteString(" for prior subagent ")
		sb.WriteString(priorID)
	}
	sb.WriteString("]\n")
	if handoff != nil {
		if handoff.Objections != "" {
			sb.WriteString("\nUnresolved objections / verification failures:\n")
			sb.WriteString(handoff.Objections)
			sb.WriteString("\n")
		}
		if handoff.Findings != "" {
			sb.WriteString("\nPrior findings / retained context:\n")
			sb.WriteString(handoff.Findings)
			sb.WriteString("\n")
		}
		if len(handoff.Files) > 0 {
			sb.WriteString("\nRelevant inspected / modified files:\n")
			for _, f := range handoff.Files {
				if trimmed := strings.TrimSpace(f); trimmed != "" {
					sb.WriteString("- ")
					sb.WriteString(trimmed)
					sb.WriteString("\n")
				}
			}
		}
	}
	sb.WriteString("\n---\nTask:\n")
	sb.WriteString(task)
	return sb.String()
}

func subagentPrompt(role, task string) string {
	if role == subagentRoleGeneral {
		return task
	}
	return "Act as the " + role + " subagent for this task.\n\n" + task
}
func permissionMode(info *Info) compozyconfig.PermissionMode {
	return compozyconfig.PermissionMode(info.EffectivePermissions)
}
func subagentInherited(info *Info) SubagentTarget {
	return SubagentTarget{
		Agent:           info.AgentName,
		Provider:        info.Provider,
		Model:           info.Model,
		ReasoningEffort: info.ReasoningEffort,
		Speed:           string(info.Speed),
		ACPOptions:      acp.CloneSessionConfigOptionSelections(info.ACPOptions),
	}
}

// resolveSubagentTarget applies provider-scoped defaults while keeping inherited
// reasoning effort attached to its original model. Explicit child effort wins.
func resolveSubagentTarget(inherited, requested, defaults SubagentTarget) SubagentTarget {
	target := requested
	if target.Agent == "" {
		target.Agent = inherited.Agent
	}
	if target.Provider == "" {
		target.Provider = inherited.Provider
	}
	base := defaults
	if target.Provider == inherited.Provider {
		base = inherited
	}
	if target.Model == "" {
		target.Model = base.Model
	}
	if target.ReasoningEffort == "" && target.Model == base.Model {
		target.ReasoningEffort = base.ReasoningEffort
	}
	if target.Speed == "" {
		target.Speed = base.Speed
		if target.Speed == "" {
			target.Speed = string(speedpkg.SpeedNormal)
		}
	}
	if target.ACPOptions == nil {
		target.ACPOptions = acp.CloneSessionConfigOptionSelections(base.ACPOptions)
	}
	return target
}

func subagentPermissions(
	info *Info,
	req SubagentRequest,
	limit SubagentPermissionLimit,
) (compozyconfig.PermissionMode, store.SessionPermissionPolicy, error) {
	mode := req.PermissionMode
	if mode == "" || mode == "inherit" {
		mode = limit.Mode
	}
	if mode.Rank() < 0 || mode.Rank() > limit.Mode.Rank() {
		return mode, store.SessionPermissionPolicy{}, fmt.Errorf(
			"%w: permission mode escalation denied",
			ErrSpawnPermissionDenied,
		)
	}
	if limit.ChildOwnsLevel && (req.PermissionMode == "" || req.PermissionMode == "inherit") {
		// An unset mode leaves the child on its own resolved level, not a copy of the limit.
		mode = ""
	}
	ceiling := store.NormalizeSessionLineage(info.ID, info.Lineage).PermissionPolicy
	policy := ceiling
	if n := req.Narrowing; n != nil {
		if n.Tools != nil {
			policy.Tools = n.Tools
			policy.MaterializedTools = false
			policy.ToolsMode = store.PermissionList
		}
		if n.Skills != nil {
			policy.Skills = n.Skills
		}
		if n.MCPServers != nil {
			policy.MCPServers = n.MCPServers
			policy.MCPServersMode = store.PermissionList
		}
		if n.WorkspacePaths != nil {
			policy.WorkspacePaths = n.WorkspacePaths
		}
	}
	policy.MaterializedMCPServers = false
	return mode, policy, ValidatePermissionSubset(ceiling, policy)
}
func presentSubagent(row store.SessionSubagent) Subagent {
	// PendingTask is recovery state, never a presentation field.
	row.PendingTask = nil
	if row.Origin == store.SubagentOriginDelegated && row.RuntimeSpeed == "" {
		row.RuntimeSpeed = string(speedpkg.SpeedNormal)
	}
	result := Subagent{SessionSubagent: row}
	if row.Result != nil {
		result.ResultPreview = subagentFirstLine(*row.Result, 280)
	}
	if row.ResultTruncated {
		result.Hint = "Read the full answer with compozy__session_history on child_session_id, type=agent_message, limit=1."
	}
	return result
}
func subagentFirstLine(text string, limit int) string {
	line := strings.TrimSpace(strings.SplitN(text, "\n", 2)[0])
	runes := []rune(line)
	if len(runes) > limit {
		return string(runes[:limit])
	}
	return line
}

func (m *Manager) validateSubagentSpawn(ctx context.Context, parent *Info, opts *SpawnOpts) error {
	active, ok := m.Get(parent.ID)
	if !ok || !active.IsPrompting() || active.promptCancellationRequested(active.CurrentTurnID()) ||
		(opts.ParentTurnID != "" && active.CurrentTurnID() != opts.ParentTurnID) {
		return ErrSubagentParentNotActive
	}
	if opts.Subagent == nil {
		return ErrSubagentCapabilityDenied
	}
	if m.instanceCeiling != nil {
		instance, err := m.instanceCeiling(ctx, parent.ID, "")
		if err != nil {
			return err
		}
		if instance != nil {
			// Delegate already bounded the mode by the child's level; the start clamps it again.
			return nil
		}
	}
	ceiling := permissionMode(parent)
	if opts.Permissions == "" {
		opts.Permissions = ceiling
	}
	if opts.Permissions.Rank() < 0 || opts.Permissions.Rank() > ceiling.Rank() {
		return ErrSpawnPermissionDenied
	}
	return nil
}
