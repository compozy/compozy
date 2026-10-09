package session

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/compozy/compozy/internal/acp"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/subagentid"
)

const subagentRoleGeneral = "general"

func normalizeSubagentRequest(req SubagentRequest) (SubagentRequest, error) {
	if strings.TrimSpace(req.Task) == "" {
		return req, invalidSubagent("task is required.")
	}
	if utf8.RuneCountInString(req.Task) > 120000 {
		return req, invalidSubagent("task exceeds 120000 characters.")
	}
	req.Title = strings.TrimSpace(req.Title)
	if req.Title == "" {
		req.Title = strings.TrimSpace(strings.SplitN(strings.TrimSpace(req.Task), "\n", 2)[0])
	}
	if utf8.RuneCountInString(req.Title) > 512 {
		return req, invalidSubagent("title exceeds 512 characters.")
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
func subagentID(parent, key string) string { return subagentid.Derive(parent, key) }

func subagentFingerprint(req SubagentRequest) (string, error) {
	// Invocation identity is outside the request semantics, allowing retries from another tool call.
	req.Caller = SubagentCaller{}
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
	if target.ReasoningEffort == "" {
		target.ReasoningEffort = base.ReasoningEffort
	}
	if target.Speed == "" {
		target.Speed = base.Speed
	}
	if target.ACPOptions == nil {
		target.ACPOptions = acp.CloneSessionConfigOptionSelections(base.ACPOptions)
	}
	return target
}

func subagentPermissions(
	info *Info,
	req SubagentRequest,
) (compozyconfig.PermissionMode, store.SessionPermissionPolicy, error) {
	mode := req.PermissionMode
	if mode == "" || mode == "inherit" {
		mode = permissionMode(info)
	}
	if mode.Rank() < 0 || mode.Rank() > permissionMode(info).Rank() {
		return mode, store.SessionPermissionPolicy{}, fmt.Errorf(
			"%w: permission mode escalation denied",
			ErrSpawnPermissionDenied,
		)
	}
	ceiling := store.NormalizeSessionLineage(info.ID, info.Lineage).PermissionPolicy
	policy := ceiling
	if n := req.Narrowing; n != nil {
		if n.Tools != nil {
			policy.Tools = n.Tools
		}
		if n.Skills != nil {
			policy.Skills = n.Skills
		}
		if n.MCPServers != nil {
			policy.MCPServers = n.MCPServers
		}
		if n.WorkspacePaths != nil {
			policy.WorkspacePaths = n.WorkspacePaths
		}
	}
	return mode, policy, ValidatePermissionSubset(ceiling, policy)
}
func presentSubagent(row store.SessionSubagent) Subagent {
	// PendingTask is recovery state, never a presentation field.
	row.PendingTask = nil
	result := Subagent{SessionSubagent: row}
	if row.Result != nil {
		result.ResultPreview = subagentFirstLine(*row.Result, 280)
	}
	if row.ResultTruncated {
		result.Hint = "Result truncated. Open the child session to read the full transcript."
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

func (m *Manager) validateSubagentSpawn(parent *Info, opts *SpawnOpts) error {
	active, ok := m.Get(parent.ID)
	if !ok || !active.IsPrompting() || active.promptCancellationRequested(active.CurrentTurnID()) ||
		(opts.ParentTurnID != "" && active.CurrentTurnID() != opts.ParentTurnID) {
		return ErrSubagentParentNotActive
	}
	if opts.Subagent == nil {
		return ErrSubagentCapabilityDenied
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
