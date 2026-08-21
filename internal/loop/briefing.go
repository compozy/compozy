package loop

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kballard/go-shellquote"
)

type BriefingTone string

const (
	BriefingToneOK       BriefingTone = "ok"
	BriefingToneNeedsYou BriefingTone = "needs_you"
	BriefingToneDegraded BriefingTone = "degraded"
	BriefingToneFailed   BriefingTone = "failed"
)

type Blocker struct {
	Kind         string     `json:"kind"`
	NodeID       NodeID     `json:"node_id,omitempty"`
	GateID       NodeID     `json:"gate_id,omitempty"`
	ItemIndex    int        `json:"item_index,omitempty"`
	WaitingSince time.Time  `json:"waiting_since"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty"`
	Expired      bool       `json:"expired,omitempty"`
	Unblocker    string     `json:"unblocker"`
}

type RunOutcome struct {
	Status    Status    `json:"status"`
	Cause     string    `json:"cause"`
	ActorKind string    `json:"actor_kind,omitempty"`
	ActorRef  string    `json:"actor_ref,omitempty"`
	At        time.Time `json:"at"`
}

type ArtifactAvailability string

const (
	ArtifactAvailable ArtifactAvailability = "available"
	ArtifactPartial   ArtifactAvailability = "partial"
	ArtifactPruned    ArtifactAvailability = "pruned"
)

type RunArtifact struct {
	Name         string               `json:"name"`
	Output       string               `json:"output,omitempty"`
	Ref          string               `json:"ref,omitempty"`
	Availability ArtifactAvailability `json:"availability"`
}

type StepProgress struct {
	Round      int `json:"round"`
	StepsDone  int `json:"steps_done"`
	StepsTotal int `json:"steps_total"`
}

type RunUsage struct {
	Tokens        int64   `json:"tokens"`
	CostUSD       float64 `json:"cost_usd,omitempty"`
	BudgetUsedPct float64 `json:"budget_used_pct,omitempty"`
	DurationMS    int64   `json:"duration_ms,omitempty"`
}

type Briefing struct {
	RunID     RunID         `json:"run_id"`
	Status    Status        `json:"status"`
	Tone      BriefingTone  `json:"tone"`
	Headline  string        `json:"headline"`
	Detail    string        `json:"detail,omitempty"`
	Blockers  []Blocker     `json:"blockers"`
	Outcome   *RunOutcome   `json:"outcome,omitempty"`
	Artifacts []RunArtifact `json:"artifacts"`
	Progress  StepProgress  `json:"progress"`
	Usage     RunUsage      `json:"usage"`
}

type BriefingSource struct {
	Run       Run
	Roster    RosterPage
	Requests  []Request
	Artifacts []RunArtifact
	Now       time.Time
}

// ProjectBriefing is the deterministic server-owned verdict projection.
func ProjectBriefing(source *BriefingSource) Briefing {
	now := source.Now.UTC()
	result := Briefing{
		RunID: source.Run.ID, Status: source.Run.Status, Tone: BriefingToneOK,
		Blockers: []Blocker{}, Artifacts: append([]RunArtifact(nil), source.Artifacts...),
		Progress: ProgressFromRoster(source.Roster, source.Run.Generation),
		Usage:    usageFromRun(source.Run, now),
	}
	result.Blockers = briefingBlockers(source, now)
	if len(result.Blockers) > 0 {
		result.Tone = blockerTone(result.Blockers[0].Kind)
		result.Headline = blockerHeadline(result.Blockers[0], len(result.Blockers))
		return result
	}
	if isTerminalStatus(source.Run.Status) {
		return terminalBriefing(result, source.Run)
	}
	switch source.Run.Status {
	case StatusQueued:
		result.Headline = "Waiting to start because the concurrency cap is full."
	case StatusWatching:
		result.Headline = "Waiting for the configured watch source."
	default:
		result.Headline = runningHeadline(source.Roster, source.Run.Generation)
	}
	return result
}

func ProgressFromRoster(roster RosterPage, generation int) StepProgress {
	progress := StepProgress{Round: generation}
	for _, node := range roster.Nodes {
		if node.Generation != generation || !node.Action || node.State == NodeStateNotTaken {
			continue
		}
		progress.StepsTotal++
		if node.State == NodeStateSucceeded || node.State == NodeStatePartial || node.State == NodeStateFailed ||
			node.State == NodeStateCanceled {
			progress.StepsDone++
		}
	}
	return progress
}

// ProgressFromRosterSource projects progress from the complete current-round roster.
func ProgressFromRosterSource(source *RosterSource) (StepProgress, error) {
	roster, err := projectCompleteRoster(source, RosterQuery{State: NodeStateFilterAll})
	if err != nil {
		return StepProgress{}, err
	}
	return ProgressFromRoster(roster, source.Run.Generation), nil
}

func briefingBlockers(source *BriefingSource, now time.Time) []Blocker {
	items := []Blocker{}
	if source.Run.Status == StatusNeedsApproval || source.Run.ActiveGateID != "" {
		items = append(items, Blocker{
			Kind: gateResultApprovalKey, GateID: source.Run.ActiveGateID,
			WaitingSince: source.Run.LastProgressAt.UTC(),
			Unblocker: shellquote.Join(
				"compozy", "loop", "approve", string(source.Run.ID),
				"--gate", string(source.Run.ActiveGateID),
				"--workspace", string(source.Run.WorkspaceID),
			),
		})
	}
	for _, node := range source.Roster.Nodes {
		if node.State == NodeStateQuarantined {
			items = append(items, nodeBlocker(string(NodeControlMutationQuarantine), source.Run, node))
		}
	}
	for _, request := range source.Requests {
		if request.State != string(joinSettlementPending) {
			continue
		}
		blocker := Blocker{
			Kind: NodeWaitKindRequest, NodeID: request.NodeID, ItemIndex: request.ItemIndex,
			WaitingSince: request.OpenedAt.UTC(), ExpiresAt: request.ExpiresAt,
			Unblocker: shellquote.Join(
				"compozy", "loop", "respond",
				"--workspace", string(source.Run.WorkspaceID),
				"--run-id", string(source.Run.ID),
				"--generation", strconv.Itoa(request.Generation),
				"--node", string(request.NodeID),
				"--item", strconv.Itoa(request.ItemIndex),
				"--decision", requestUnblockerDecision(request),
				"--payload", "<json>",
			),
		}
		blocker.Expired = request.ExpiresAt != nil && !request.ExpiresAt.After(now)
		items = append(items, blocker)
	}
	for _, node := range source.Roster.Nodes {
		if node.State == NodeStateFailed {
			items = append(items, nodeBlocker(namespaceFailureKey, source.Run, node))
		}
		if node.State == NodeStateRetrying {
			items = append(items, Blocker{
				Kind: "backoff", NodeID: node.NodeID, ItemIndex: node.ItemIndex,
				WaitingSince: timeOrZero(node.EndedAt), Unblocker: "",
			})
		}
	}
	order := map[string]int{
		gateResultApprovalKey: 0, string(NodeControlMutationQuarantine): 1, NodeWaitKindRequest: 2,
		namespaceFailureKey: 3, "backoff": 4, "quota": 4,
	}
	sort.SliceStable(items, func(i, j int) bool {
		return order[items[i].Kind] < order[items[j].Kind]
	})
	return items
}

func requestUnblockerDecision(request Request) string {
	if request.Kind == RequestKindAsk {
		return RequestDecisionRespond
	}
	if len(request.Decisions) == 1 {
		return request.Decisions[0]
	}
	return "<decision>"
}

func nodeBlocker(kind string, run Run, node RosterNode) Blocker {
	waitingSince := timeOrZero(node.StartedAt)
	if kind == namespaceFailureKey {
		waitingSince = timeOrZero(node.EndedAt)
	}
	return Blocker{
		Kind: kind, NodeID: node.NodeID, ItemIndex: node.ItemIndex,
		WaitingSince: waitingSince,
		Unblocker: shellquote.Join(
			"compozy", "loop", "node", "requeue",
			"--workspace", string(run.WorkspaceID),
			"--run-id", string(run.ID),
			"--node", string(node.NodeID),
		),
	}
}

func blockerTone(kind string) BriefingTone {
	switch kind {
	case gateResultApprovalKey, string(NodeControlMutationQuarantine), NodeWaitKindRequest:
		return BriefingToneNeedsYou
	case namespaceFailureKey:
		return BriefingToneFailed
	default:
		return BriefingToneDegraded
	}
}

func blockerHeadline(blocker Blocker, count int) string {
	label := strings.ReplaceAll(blocker.Kind, "_", " ")
	if count == 1 {
		return fmt.Sprintf("This run needs attention: %s.", label)
	}
	return fmt.Sprintf("This run has %d blockers; first: %s.", count, label)
}

func runningHeadline(roster RosterPage, generation int) string {
	for _, node := range roster.Nodes {
		if node.Generation == generation && node.State == NodeStateRunning {
			return fmt.Sprintf("Running step %s in round %d.", node.NodeID, generation)
		}
	}
	return fmt.Sprintf("Run is active in round %d.", generation)
}

func terminalBriefing(result Briefing, run Run) Briefing {
	cause := string(run.Status)
	at := run.LastProgressAt.UTC()
	result.Outcome = &RunOutcome{Status: run.Status, Cause: cause, At: at}
	if run.Status == StatusCanceled {
		result.Outcome.ActorKind = string(run.ControlActor.Kind)
		result.Outcome.ActorRef = run.ControlActor.Ref
	}
	switch run.Status {
	case StatusFailed, StatusExhausted, StatusStalled:
		result.Tone = BriefingToneFailed
	default:
		result.Tone = BriefingToneOK
	}
	if run.Status == StatusNoOp {
		result.Headline = "Run finished without producing outputs."
		result.Artifacts = []RunArtifact{}
	} else {
		result.Headline = terminalHeadline(*result.Outcome, result.Artifacts)
	}
	return result
}

func terminalHeadline(outcome RunOutcome, artifacts []RunArtifact) string {
	base := fmt.Sprintf("Run finished: %s.", outcome.Status)
	if outcome.ActorKind != "" {
		base = fmt.Sprintf(
			"Run %s by %s %s at %s.",
			outcome.Status,
			outcome.ActorKind,
			outcome.ActorRef,
			outcome.At.Format(time.RFC3339),
		)
	}
	names := []string{}
	for _, item := range artifacts {
		names = append(names, item.Name)
	}
	if len(names) > 0 {
		base += " Produced: " + strings.Join(names, ", ") + "."
	}
	return base
}

func usageFromRun(run Run, now time.Time) RunUsage {
	usage := RunUsage{Tokens: run.TokensUsed}
	if run.BudgetTokens > 0 {
		usage.BudgetUsedPct = float64(run.TokensUsed) / float64(run.BudgetTokens) * 100
	}
	start := run.StartedAt
	if start.IsZero() {
		start = run.CreatedAt
	}
	if !start.IsZero() {
		end := now
		if isTerminalStatus(run.Status) && !run.LastProgressAt.IsZero() {
			end = run.LastProgressAt
		}
		usage.DurationMS = end.Sub(start).Milliseconds()
	}
	return usage
}

func isTerminalStatus(status Status) bool {
	switch status {
	case StatusDone, StatusNoOp, StatusBlocked, StatusFailed, StatusExhausted, StatusStalled, StatusCanceled:
		return true
	default:
		return false
	}
}

func timeOrZero(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}
