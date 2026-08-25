package calls

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/contracts"
	"github.com/compozy/compozy/internal/network/participation"
	"github.com/compozy/compozy/internal/task"
)

func (s *Service) Create(ctx context.Context, input CreateInput) (CallRecord, error) {
	normalized, target, roster, err := s.normalizeCreate(ctx, input)
	if err != nil {
		return CallRecord{}, err
	}
	prepared, err := s.prepareAdmission(normalized, target)
	if err != nil {
		return CallRecord{}, err
	}
	result, err := s.store.AdmitCall(ctx, prepared)
	if err != nil {
		return CallRecord{}, err
	}
	result.Record.Replayed = result.Replayed
	if result.Replayed {
		return result.Record, nil
	}
	s.emitHook(ctx, HookCallCreated, hookPayloadForCall(result.Record))
	if result.Record.ActivationRunID == "" {
		return result.Record, nil
	}
	activated, err := s.activateFastPath(ctx, result.Record, prepared, roster)
	if err != nil {
		return CallRecord{}, err
	}
	return activated, nil
}

func (s *Service) CreateBatch(ctx context.Context, inputs []CreateInput) ([]BatchOutcome, error) {
	if len(inputs) == 0 {
		return nil, newError(CodeBatchEmpty, "at least one call is required", nil)
	}
	if len(inputs) > s.config.MaxBatch {
		return nil, newError(
			CodeBatchOverCap,
			fmt.Sprintf("batch has %d items; maximum is %d", len(inputs), s.config.MaxBatch),
			nil,
		)
	}
	batchID, err := s.newID("batch")
	if err != nil {
		return nil, fmt.Errorf("calls: generate batch id: %w", err)
	}
	outcomes := make([]BatchOutcome, len(inputs))
	for index := range inputs {
		item := inputs[index]
		item.BatchID = batchID
		record, createErr := s.Create(ctx, item)
		if createErr != nil {
			outcomes[index].Error = createErr
			continue
		}
		outcomes[index].Call = &record
	}
	return outcomes, nil
}

func (s *Service) normalizeCreate(
	ctx context.Context,
	input CreateInput,
) (CreateInput, TargetContext, []AgentRosterEntry, error) {
	in := input
	in.ProfileID = strings.TrimSpace(in.ProfileID)
	in.WorkspaceID = strings.TrimSpace(in.WorkspaceID)
	in.Target.Agent = strings.TrimSpace(in.Target.Agent)
	in.Target.SessionID = strings.TrimSpace(in.Target.SessionID)
	in.IdempotencyKey = strings.TrimSpace(in.IdempotencyKey)
	in.Actor.Kind = strings.TrimSpace(in.Actor.Kind)
	in.Actor.ID = strings.TrimSpace(in.Actor.ID)
	in.Caller.ID = strings.TrimSpace(in.Caller.ID)
	in.Caller.WorkspaceID = strings.TrimSpace(in.Caller.WorkspaceID)
	cleanPrompt, _, rejectPrompt := contracts.SanitizeText(in.Prompt)
	if rejectPrompt {
		return CreateInput{}, TargetContext{}, nil, newError(
			CodeValidation,
			"call prompt contains unsafe secret material",
			nil,
		)
	}
	in.Prompt = cleanPrompt
	if in.Scope == "" {
		if in.WorkspaceID == "" {
			in.Scope = ScopeGlobal
		} else {
			in.Scope = ScopeWorkspace
		}
	}
	if err := validateCreateIdentity(in); err != nil {
		return CreateInput{}, TargetContext{}, nil, err
	}
	if in.IdleTTL == 0 {
		in.IdleTTL = s.idleTTL
	}
	if in.IdleTTL <= 0 {
		return CreateInput{}, TargetContext{}, nil, newError(CodeValidation, "idle_ttl must be positive", nil)
	}
	if in.Deadline != nil {
		deadline := in.Deadline.UTC()
		if !deadline.After(s.now().UTC()) {
			return CreateInput{}, TargetContext{}, nil, newError(CodeDeadlineInvalid, "deadline must be in the future", nil)
		}
		in.Deadline = &deadline
	}
	if in.Runtime != nil {
		runtime := in.Runtime.Normalize()
		in.Runtime = &runtime
	}
	target, roster, err := s.directory.ResolveCallTarget(ctx, in)
	if err != nil {
		return CreateInput{}, TargetContext{}, roster, err
	}
	if err := validateTargetContext(in, target, roster, s.config.MaxDepth, s.config.MaxChildren); err != nil {
		return CreateInput{}, TargetContext{}, roster, err
	}
	if widening := wideningPermissionAtoms(target.CallerPolicy, in.Narrow.Policy()); len(widening) > 0 {
		return CreateInput{}, TargetContext{}, roster, &Error{
			Code: CodeWideningRejected, Message: "permission narrowing widens the caller set",
			Widening: widening,
		}
	}
	return in, target, roster, nil
}

func validateCreateIdentity(in CreateInput) error {
	switch {
	case in.ProfileID == "":
		return newError(CodeValidation, "profile_id is required", nil)
	case strings.TrimSpace(in.Prompt) == "":
		return newError(CodePromptRequired, "prompt is required", nil)
	case (in.Target.Agent == "") == (in.Target.SessionID == ""):
		return newError(CodeValidation, "exactly one target is required", nil)
	case in.Actor.Kind == "" || in.Actor.ID == "":
		return newError(CodeValidation, "actor kind and id are required", nil)
	case in.Caller.ID == "":
		return newError(CodeValidation, "caller id is required", nil)
	case in.Caller.Kind != participation.OwnerKindSession &&
		in.Caller.Kind != participation.OwnerKindTaskRun &&
		in.Caller.Kind != participation.OwnerKindLoopRun &&
		in.Caller.Kind != participation.OwnerKindAutomationRun:
		return newError(CodeValidation, fmt.Sprintf("unsupported caller kind %q", in.Caller.Kind), nil)
	case in.Scope == ScopeWorkspace && in.WorkspaceID == "":
		return newError(CodeValidation, "workspace scope requires workspace_id", nil)
	case in.Scope == ScopeGlobal && in.WorkspaceID != "":
		return newError(CodeValidation, "global scope requires an empty workspace_id", nil)
	case in.Scope != ScopeGlobal && in.Scope != ScopeWorkspace:
		return newError(CodeValidation, fmt.Sprintf("unsupported scope %q", in.Scope), nil)
	case in.Scope == ScopeWorkspace && in.Caller.WorkspaceID != in.WorkspaceID:
		return newError(CodeWorkspaceDenied, "caller workspace does not match call workspace", nil)
	default:
		return nil
	}
}

func validateTargetContext(
	in CreateInput,
	target TargetContext,
	roster []AgentRosterEntry,
	maxDepth int,
	maxChildren int,
) error {
	if in.Target.Agent != "" && strings.TrimSpace(target.AgentName) == "" {
		names := make([]string, 0, len(roster))
		for _, item := range roster {
			names = append(names, strings.TrimSpace(item.Name))
		}
		sort.Strings(names)
		return &Error{
			Code:      CodeAgentUnknown,
			Message:   fmt.Sprintf("agent %q was not found; available: %s", in.Target.Agent, strings.Join(names, ", ")),
			Available: append([]AgentRosterEntry(nil), roster...),
		}
	}
	if strings.TrimSpace(target.ProfileID) != in.ProfileID {
		return newError(CodeTargetDenied, "target belongs to another profile", nil)
	}
	if in.Scope == ScopeWorkspace && strings.TrimSpace(target.WorkspaceID) != in.WorkspaceID {
		return newError(CodeWorkspaceDenied, "target belongs to another workspace", nil)
	}
	if !target.Allowed {
		return newError(CodeTargetDenied, "target is outside the caller lineage", nil)
	}
	if in.Target.Agent != "" && target.Depth > maxDepth {
		return newError(CodeDepthExceeded, fmt.Sprintf("call depth %d exceeds maximum %d", target.Depth, maxDepth), nil)
	}
	if in.Target.Agent != "" && target.LiveChildren >= maxChildren {
		return newError(CodeChildrenCap, fmt.Sprintf("parent has %d live children; maximum is %d", target.LiveChildren, maxChildren), nil)
	}
	if in.Target.SessionID != "" {
		switch strings.TrimSpace(target.State) {
		case "expired":
			expiredAt := target.ExpiredAt.UTC().Format(time.RFC3339Nano)
			return &Error{
				Code: CodeTargetExpired, Message: fmt.Sprintf("target expired at %s; call the agent fresh", expiredAt),
				ExpiredAt: expiredAt, Suggestion: "call the agent fresh",
			}
		case "missing", "":
			return newError(CodeTargetNotFound, fmt.Sprintf("target session %q was not found", in.Target.SessionID), nil)
		}
	}
	return nil
}

func (s *Service) prepareAdmission(in CreateInput, target TargetContext) (Admission, error) {
	var contract *contracts.Contract
	if len(in.Expect) > 0 {
		prepared, err := contracts.Prepare(in.Expect)
		if err != nil {
			return Admission{}, newError(CodeExpectInvalid, err.Error(), err)
		}
		contract = &prepared
	}
	budget, err := contracts.ResolveBudget(in.ResultBudget, s.resultPolicy)
	if err != nil {
		return Admission{}, newError(CodeValidation, err.Error(), err)
	}
	callID, err := s.newID("call")
	if err != nil {
		return Admission{}, fmt.Errorf("calls: generate call id: %w", err)
	}
	now := s.now().UTC()
	runtime := target.Runtime
	if in.Runtime != nil {
		runtime = *in.Runtime
	}
	record := CallRecord{
		CallID: callID, ProfileID: in.ProfileID, Scope: in.Scope, WorkspaceID: in.WorkspaceID,
		Caller: in.Caller, Actor: in.Actor, ParentSessionID: target.ParentSessionID,
		AgentName: target.AgentName, ChildSessionID: target.ChildSessionID,
		GovernedRootID: target.GovernedRootID, Depth: target.Depth, State: StateQueued,
		PromptRef: contracts.OutputRefForPayload(json.RawMessage(in.Prompt)), ResultBudget: budget,
		Strict: in.Strict, IdleTTL: in.IdleTTL, Runtime: runtime,
		IdempotencyKey: in.IdempotencyKey, BatchID: in.BatchID, CreatedAt: now, UpdatedAt: now,
	}
	if contract != nil {
		record.ExpectDigest = contract.Digest
	}
	if in.Deadline != nil {
		record.DeadlineAt = *in.Deadline
	}
	activation, followUp, err := s.activationFor(record, target)
	if err != nil {
		return Admission{}, err
	}
	if activation != nil {
		record.ActivationRunID = activation.RunID
	} else if followUp != nil {
		record.State = StateRunning
		record.StartedAt = now
		followUp.Body = in.Prompt
	}
	record.RequestDigest, err = requestDigest(record, in.Prompt, in.Narrow)
	if err != nil {
		return Admission{}, err
	}
	return Admission{
		Record: record, Contract: contract, Prompt: []byte(in.Prompt), MaxChildren: s.config.MaxChildren,
		Permissions: flattenPermissions(in.Narrow), Narrow: in.Narrow,
		Activation: activation, FollowUp: followUp,
	}, nil
}

func requestDigest(record CallRecord, prompt string, narrow PermissionAtoms) (string, error) {
	identity := struct {
		ProfileID, Scope, WorkspaceID, CallerKind, CallerID string
		TargetAgent, TargetSession, Prompt, ExpectDigest    string
		Budget                                              int
		Overflow                                            string
		Strict                                              bool
		TTL                                                 int64
		Deadline                                            string
		Runtime                                             RuntimeSpec
		Permissions                                         []string
	}{
		ProfileID: record.ProfileID, Scope: string(record.Scope), WorkspaceID: record.WorkspaceID,
		CallerKind: string(record.Caller.Kind), CallerID: record.Caller.ID,
		TargetAgent: record.AgentName, TargetSession: record.ChildSessionID,
		Prompt: prompt, ExpectDigest: record.ExpectDigest, Budget: record.ResultBudget.MaxBytes,
		Overflow: string(record.ResultBudget.Overflow), Strict: record.Strict,
		TTL: int64(record.IdleTTL), Runtime: record.Runtime, Permissions: flattenPermissions(narrow),
	}
	if !record.DeadlineAt.IsZero() {
		identity.Deadline = record.DeadlineAt.UTC().Format(time.RFC3339Nano)
	}
	encoded, err := json.Marshal(identity)
	if err != nil {
		return "", fmt.Errorf("calls: encode request identity: %w", err)
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func flattenPermissions(atoms PermissionAtoms) []string {
	groups := []struct {
		name   string
		values []string
	}{
		{"tools", atoms.Tools}, {"skills", atoms.Skills}, {"mcp_servers", atoms.MCPServers},
		{"workspace_paths", atoms.WorkspacePaths}, {"network_channels", atoms.NetworkChannels},
		{"sandbox_profiles", atoms.SandboxProfiles},
	}
	result := make([]string, 0)
	for _, group := range groups {
		for _, value := range group.values {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				result = append(result, group.name+":"+trimmed)
			}
		}
	}
	sort.Strings(result)
	return result
}

func (s *Service) activationFor(record CallRecord, target TargetContext) (*ActivationSpec, *Delivery, error) {
	if record.AgentName == "" && strings.TrimSpace(target.State) == "active" {
		return nil, &Delivery{CallID: record.CallID, RecipientSessionID: record.ChildSessionID, Kind: "message"}, nil
	}
	runID, err := s.newID("run")
	if err != nil {
		return nil, nil, fmt.Errorf("calls: generate activation run id: %w", err)
	}
	kind := "spawn"
	if record.AgentName == "" {
		kind = "revive"
	}
	return &ActivationSpec{
		RunID: runID, CallID: record.CallID, WorkspaceID: record.WorkspaceID,
		GovernedRootID: record.GovernedRootID, Kind: kind,
		ParentSessionID: record.ParentSessionID, TargetSessionID: record.ChildSessionID,
		AgentName: record.AgentName, Depth: record.Depth, IdleTTL: record.IdleTTL, Runtime: record.Runtime,
	}, nil, nil
}

func (s *Service) activateFastPath(
	ctx context.Context,
	record CallRecord,
	admission Admission,
	_ []AgentRosterEntry,
) (CallRecord, error) {
	if s.claimer == nil || s.invoker == nil || admission.Activation == nil {
		return record, nil
	}
	actor := activationDaemonActor(record.WorkspaceID)
	claim, err := s.claimer.ClaimNextRun(ctx, task.ClaimCriteria{
		RunID: record.ActivationRunID, RunKind: task.RunKindCallActivation,
		Scope: task.Scope(record.Scope), WorkspaceID: record.WorkspaceID,
	}, actor)
	if err != nil {
		if err == task.ErrNoClaimableRun {
			return record, nil
		}
		return CallRecord{}, fmt.Errorf("calls: claim activation %q: %w", record.ActivationRunID, err)
	}
	return s.invokeClaimedActivation(ctx, record, admission, claim)
}
