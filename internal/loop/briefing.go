package loop

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/kballard/go-shellquote"
)

// BriefingTone classifies the run verdict for public read surfaces.
type BriefingTone string

const (
	BriefingToneOK       BriefingTone = "ok"
	BriefingToneNeedsYou BriefingTone = "needs_you"
	BriefingToneDegraded BriefingTone = "degraded"
	BriefingToneFailed   BriefingTone = "failed"
	unknownValue                      = "unknown"
)

// Blocker describes one ordered condition preventing progress and its exact unblocker.
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

// RunOutcome carries the cause-aware terminal settlement and its durable timestamp.
type RunOutcome struct {
	Status    Status    `json:"status"`
	Cause     string    `json:"cause"`
	ActorKind string    `json:"actor_kind,omitempty"`
	ActorRef  string    `json:"actor_ref,omitempty"`
	At        time.Time `json:"at"`
}

// ArtifactAvailability states whether produced content can still be read.
type ArtifactAvailability string

const (
	ArtifactAvailable ArtifactAvailability = "available"
	ArtifactPartial   ArtifactAvailability = "partial"
	ArtifactPruned    ArtifactAvailability = "pruned"
)

// RunArtifact identifies one logical output and its content reference.
type RunArtifact struct {
	Name         string               `json:"name"`
	Output       string               `json:"output,omitempty"`
	Ref          string               `json:"ref,omitempty"`
	Availability ArtifactAvailability `json:"availability"`
}

// StepProgress summarizes current-round action completion.
type StepProgress struct {
	Round      int `json:"round"`
	StepsDone  int `json:"steps_done"`
	StepsTotal int `json:"steps_total"`
}

// RunUsage summarizes token, cost, budget, and elapsed-time usage.
type RunUsage struct {
	Tokens        int64   `json:"tokens"`
	CostUSD       float64 `json:"cost_usd,omitempty"`
	BudgetUsedPct float64 `json:"budget_used_pct,omitempty"`
	Duration      string  `json:"duration,omitempty"`
}

// Briefing is the server-owned verdict shared by UI, CLI, HTTP, UDS, and tools.
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

// BriefingSource contains the durable facts required to project one briefing.
type BriefingSource struct {
	Graph                dsl.Graph
	Outputs              []GenerationOutput
	Run                  Run
	Roster               RosterPage
	Requests             []Request
	Artifacts            []RunArtifact
	Outcome              *RunOutcome
	ApprovalWaitingSince time.Time
	Now                  time.Time
}

// ProjectBriefing is the deterministic server-owned verdict projection.
func ProjectBriefing(source *BriefingSource) Briefing {
	now := source.Now.UTC()
	result := Briefing{
		RunID: source.Run.ID, Status: source.Run.Status, Tone: BriefingToneOK,
		Blockers: []Blocker{}, Artifacts: append([]RunArtifact{}, source.Artifacts...),
		Progress: ProgressFromRoster(source.Roster, currentBriefingGeneration(source)),
		Usage:    usageFromRun(source.Run, now),
	}
	result.Blockers = briefingBlockers(source, now)
	if isTerminalStatus(source.Run.Status) {
		return terminalBriefing(result, source)
	}
	if len(result.Blockers) > 0 {
		result.Tone = blockerTone(result.Blockers[0].Kind)
		result.Headline = blockerHeadline(result.Blockers[0], len(result.Blockers))
		result.Detail = progressDetail(result.Progress)
		return result
	}
	switch source.Run.Status {
	case StatusQueued:
		result.Headline = "Waiting to start because the concurrency cap is full."
		result.Detail = "The run will start when an execution slot is available."
	case StatusWatching:
		result.Headline = "Waiting for the configured watch source."
		result.Detail = "No new matching input has arrived."
	default:
		result.Headline = runningHeadline(source.Roster, result.Progress.Round)
		result.Detail = progressDetail(result.Progress)
	}
	return result
}

func currentBriefingGeneration(source *BriefingSource) int {
	generation := source.Run.Generation
	for _, node := range source.Roster.Nodes {
		generation = max(generation, node.Generation)
	}
	return generation
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
	roster, err := projectCompleteRoster(source, NodeStateFilterAll, 0)
	if err != nil {
		return StepProgress{}, err
	}
	return ProgressFromRoster(roster, source.Run.Generation), nil
}

func briefingBlockers(source *BriefingSource, now time.Time) []Blocker {
	generation := currentBriefingGeneration(source)
	items := []Blocker{}
	if source.Run.Status == StatusNeedsApproval || source.Run.ActiveGateID != "" {
		items = append(items, Blocker{
			Kind: gateResultApprovalKey, GateID: source.Run.ActiveGateID,
			WaitingSince: source.ApprovalWaitingSince.UTC(),
			Unblocker: shellquote.Join(
				"compozy", "loop", "approve", string(source.Run.ID),
				"--workspace", string(source.Run.WorkspaceID),
				"--gate", string(source.Run.ActiveGateID),
			),
		})
	}
	for _, node := range source.Roster.Nodes {
		if node.Generation != generation {
			continue
		}
		if node.State == NodeStateQuarantined {
			items = append(items, nodeBlocker(string(NodeControlMutationQuarantine), source, node))
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
				"--payload-stdin",
			),
		}
		blocker.Expired = request.ExpiresAt != nil && !request.ExpiresAt.After(now)
		items = append(items, blocker)
	}
	for _, node := range source.Roster.Nodes {
		if node.Generation != generation {
			continue
		}
		if node.State == NodeStateFailed {
			items = append(items, nodeBlocker(namespaceFailureKey, source, node))
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

func nodeBlocker(kind string, source *BriefingSource, node RosterNode) Blocker {
	run := source.Run
	waitingSince := timeOrZero(node.StartedAt)
	if kind == namespaceFailureKey {
		waitingSince = timeOrZero(node.EndedAt)
	}
	blocker := Blocker{Kind: kind, NodeID: node.NodeID, ItemIndex: node.ItemIndex, WaitingSince: waitingSince}
	if kind == namespaceFailureKey {
		blocker.Unblocker = failureRerunCommand(source, node)
	} else if !run.Status.Terminal() {
		blocker.Unblocker = shellquote.Join(
			"compozy", "loop", "node", "requeue",
			"--workspace", string(run.WorkspaceID),
			"--run-id", string(run.ID),
			"--node", string(node.NodeID),
		)
	}
	return blocker
}

func failureRerunCommand(source *BriefingSource, node RosterNode) string {
	run := source.Run
	run.Generation = currentBriefingGeneration(source)
	if !run.Status.Terminal() || node.Generation != run.Generation {
		return ""
	}
	outputs := make([]GenerationOutput, 0)
	for _, output := range source.Outputs {
		if output.Generation == run.Generation {
			outputs = append(outputs, output)
		}
	}
	// Advertise only a rerun the existing planner accepts, including pending dependents.
	_, labels, err := planOperatorRerun(source.Graph, outputs, node.NodeID, &node.ItemIndex, run.Generation+1)
	if err != nil || validateTerminalRerunOutputs(outputs, labels) != nil {
		return ""
	}
	return shellquote.Join(
		"compozy", "loop", "rerun",
		"--workspace", string(run.WorkspaceID),
		"--run-id", string(run.ID),
		"--from-node", string(node.NodeID),
		"--item", strconv.Itoa(node.ItemIndex),
	)
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

func terminalBriefing(result Briefing, source *BriefingSource) Briefing {
	run := source.Run
	result.Artifacts = labelTerminalArtifacts(result.Artifacts)
	if source.Outcome != nil {
		outcome := *source.Outcome
		result.Outcome = &outcome
	} else {
		result.Outcome = &RunOutcome{Status: run.Status, Cause: unknownValue}
	}
	if run.Status == StatusCanceled && result.Outcome.ActorKind == "" {
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
		result.Headline = terminalHeadline(*result.Outcome)
	}
	result.Detail = progressDetail(result.Progress)
	return result
}

func labelTerminalArtifacts(artifacts []RunArtifact) []RunArtifact {
	for index := range artifacts {
		label := strings.TrimSpace(artifacts[index].Name)
		if label == "" {
			label = strings.TrimSpace(artifacts[index].Output)
		}
		if label == "" {
			label = fmt.Sprintf("output %d", index+1)
		}
		artifacts[index].Name = label
	}
	return artifacts
}

func terminalHeadline(outcome RunOutcome) string {
	if outcome.ActorKind != "" {
		return fmt.Sprintf("Run %s by %s %s.", outcome.Status, outcome.ActorKind, outcome.ActorRef)
	}
	return fmt.Sprintf("Run finished: %s.", outcome.Status)
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
		usage.Duration = formatBriefingDuration(end.Sub(start))
	}
	return usage
}

func formatBriefingDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}
	return duration.Round(time.Second).String()
}

func progressDetail(progress StepProgress) string {
	return fmt.Sprintf(
		"%d of %d steps are complete in round %d.",
		progress.StepsDone,
		progress.StepsTotal,
		progress.Round,
	)
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
