// Package gate evaluates loop verification gates.
package gate

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
	"github.com/compozy/agh/internal/task"
	"github.com/compozy/agh/internal/tools"
)

const (
	// DefaultBrokenJudgeStreakLimit is the ADR-012 fail-open guard.
	DefaultBrokenJudgeStreakLimit = 3
)

var (
	// ErrInvalidGate reports an authored gate that cannot be evaluated.
	ErrInvalidGate = errors.New("loop gate: invalid gate")
	// ErrMaxRevisionsCeiling reports a gate above the hard revision ceiling.
	ErrMaxRevisionsCeiling = errors.New("loop gate: max_revisions exceeds ceiling")
	// ErrVerdictPolicyRequiresJudge reports a revise policy without a verdict source.
	ErrVerdictPolicyRequiresJudge = errors.New("loop gate: verdict_policy revise_until_clean requires judge or human")
)

// GateEvaluator evaluates one gate criteria bundle and returns its verdict and route.
//
//revive:disable-next-line:exported // GateEvaluator is the TechSpec contract name.
type GateEvaluator interface {
	Evaluate(ctx context.Context, gate Gate, in GateInput) (Verdict, error)
}

// Placement identifies where a gate verdict is consumed.
type Placement string

const (
	// PlacementInBody is a gate control node inside the graph body.
	PlacementInBody Placement = "in_body"
	// PlacementDefinitionOfDone is the loop contract gate between generations.
	PlacementDefinitionOfDone Placement = "definition_of_done"
)

// Gate is the runtime shape of a control gate or definition-of-done gate.
type Gate struct {
	ID            string
	Criteria      []dsl.GateCriterion
	VerdictPolicy dsl.VerdictPolicy
	// OnResult maps verdict result keys to route overrides. Recognized keys are
	// pass, approval, fail, blocked, error, timeout, and invalid_output.
	OnResult     map[string]any
	MaxRevisions int
}

// GateFromNode adapts a DSL gate node into the evaluator input.
//
//revive:disable-next-line:exported // GateFromNode is the explicit adapter name for the gate package.
func GateFromNode(node dsl.Node) Gate {
	return Gate{
		ID:            string(node.ID),
		Criteria:      cloneCriteria(node.Criteria),
		VerdictPolicy: node.VerdictPolicy,
		OnResult:      cloneResultRoutes(node.OnResult),
		MaxRevisions:  node.MaxRevisions,
	}
}

// GateFromContract adapts contract verification criteria into a DoD gate.
//
//revive:disable-next-line:exported // GateFromContract is the explicit adapter name for the gate package.
func GateFromContract(id string, contract dsl.Contract, maxRevisions int) Gate {
	return Gate{
		ID:            strings.TrimSpace(id),
		Criteria:      cloneCriteria(contract.Verification),
		VerdictPolicy: contractVerdictPolicy(contract.Verification),
		MaxRevisions:  maxRevisions,
	}
}

// GateInput carries runtime state required to evaluate and route a gate.
//
//revive:disable-next-line:exported // GateInput is the TechSpec contract name.
type GateInput struct {
	LoopRunID                string
	Placement                Placement
	Contract                 *dsl.Contract
	TemplateData             map[string]any
	Revision                 int
	BrokenJudgeStreak        int
	BrokenJudgeStreakLimit   int
	HumanDecisions           map[string]HumanDecision
	ToolScope                tools.Scope
	ToolCallCorrelationID    string
	ToolSensitiveInputFields []string
	JudgeModel               string
	JudgeUsageReporter       JudgeUsageReporter
	JudgeEvidence            JudgeEvidence
	NetworkParticipation     *participation.Spec
}

// JudgeEvidence is the authoritative completed candidate supplied to an agent judge.
type JudgeEvidence struct {
	Text       string
	Structured json.RawMessage
}

// HumanDecision captures one human gate decision made through a trusted actor context.
type HumanDecision struct {
	Decision HumanDecisionKind
	Actor    task.ActorContext
	Note     string
}

// HumanDecisionKind is the closed approval vocabulary for human criteria.
type HumanDecisionKind string

const (
	// HumanDecisionApprove resumes the passing path.
	HumanDecisionApprove HumanDecisionKind = "approve"
	// HumanDecisionRequestChanges routes to revise or next generation.
	HumanDecisionRequestChanges HumanDecisionKind = "request_changes"
	// HumanDecisionReject routes to terminal halt.
	HumanDecisionReject HumanDecisionKind = "reject"
)

// VerdictOutcome is the canonical criterion and aggregate verdict taxonomy.
type VerdictOutcome string

const (
	// VerdictOutcomeApproved accepts the gate.
	VerdictOutcomeApproved VerdictOutcome = "approved"
	// VerdictOutcomeRejected rejects the gate and requests revision.
	VerdictOutcomeRejected VerdictOutcome = "rejected"
	// VerdictOutcomeAwaitingApproval parks the gate until a human decision arrives.
	VerdictOutcomeAwaitingApproval VerdictOutcome = "awaiting_approval"
	// VerdictOutcomeBlocked reports an explicit external blocker.
	VerdictOutcomeBlocked VerdictOutcome = "blocked"
	// VerdictOutcomeError reports a checker execution error.
	VerdictOutcomeError VerdictOutcome = "error"
	// VerdictOutcomeTimeout reports a checker timeout.
	VerdictOutcomeTimeout VerdictOutcome = "timeout"
	// VerdictOutcomeInvalidOutput reports a responsive checker with unusable output.
	VerdictOutcomeInvalidOutput VerdictOutcome = "invalid_output"
)

// CriterionResult is the observable per-criterion outcome for SSE/UI consumers.
type CriterionResult struct {
	ID                  string              `json:"id"`
	Type                dsl.CriterionType   `json:"type"`
	Outcome             VerdictOutcome      `json:"outcome"`
	Passed              bool                `json:"passed"`
	Broken              bool                `json:"broken,omitempty"`
	ExitCode            *int                `json:"exit_code,omitempty"`
	Stdout              string              `json:"stdout,omitempty"`
	Stderr              string              `json:"stderr,omitempty"`
	Confidence          *float64            `json:"confidence,omitempty"`
	Evidence            json.RawMessage     `json:"evidence,omitempty"`
	BlockingIssues      []BlockingIssue     `json:"blocking_issues,omitempty"`
	Warnings            []DiagnosticWarning `json:"warnings,omitempty"`
	Payload             json.RawMessage     `json:"payload,omitempty"`
	judgeTokensUsed     int64
	judgeTokensReported bool
}

// BlockingIssue is the structured blocker ID shape from ADR-022.
type BlockingIssue struct {
	ID   string `json:"id"`
	Note string `json:"note"`
}

// DiagnosticWarning is a non-terminal evaluator diagnostic.
type DiagnosticWarning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RouteAction is the closed routing vocabulary produced by the evaluator.
type RouteAction string

const (
	// RouteContinue advances past an in-body gate.
	RouteContinue RouteAction = "continue"
	// RouteRevise re-enters the body revision path.
	RouteRevise RouteAction = "revise"
	// RouteBranch selects a named branch.
	RouteBranch RouteAction = "branch"
	// RouteHalt stops the loop.
	RouteHalt RouteAction = "halt"
	// RouteEscalate parks for human approval.
	RouteEscalate RouteAction = "escalate"
	// RouteDone marks a definition-of-done gate complete.
	RouteDone RouteAction = "done"
	// RouteNextGeneration schedules the next generation.
	RouteNextGeneration RouteAction = "next_generation"
)

// RouteDecision is the routing result for the gate placement.
type RouteDecision struct {
	Placement      Placement   `json:"placement"`
	Action         RouteAction `json:"action"`
	Branch         string      `json:"branch,omitempty"`
	TerminalStatus string      `json:"terminal_status,omitempty"`
	ReasonCode     string      `json:"reason_code,omitempty"`
}

// Verdict is the aggregate gate verdict.
type Verdict struct {
	Outcome        VerdictOutcome    `json:"outcome"`
	Criteria       []CriterionResult `json:"criteria"`
	BlockingIssues []BlockingIssue   `json:"blocking_issues,omitempty"`
	// Payload carries the first aggregate verdict-source evidence or payload.
	Payload json.RawMessage `json:"payload,omitempty"`
	// Broken marks fail-open transport failure; consumers must use Route for control flow.
	Broken                 bool                `json:"broken,omitempty"`
	Warnings               []DiagnosticWarning `json:"warnings,omitempty"`
	Route                  RouteDecision       `json:"route"`
	NextBrokenJudgeStreak  int                 `json:"next_broken_judge_streak,omitempty"`
	BlockingIssueSignature []string            `json:"blocking_issue_signature,omitempty"`
}

// CommandRunner executes command criteria.
type CommandRunner interface {
	RunCommand(ctx context.Context, req CommandRequest) (CommandResult, error)
}

// CommandRequest is one command criterion invocation.
type CommandRequest struct {
	GateID      string
	CriterionID string
	Command     string
	Expect      string
	Contains    string
}

// CommandResult captures the deterministic command result.
type CommandResult struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

// JudgeRunner executes agent-judge criteria.
type JudgeRunner interface {
	Judge(ctx context.Context, req JudgeRequest) (JudgeResponse, error)
}

// JudgeRequest is the rendered agent-judge invocation.
type JudgeRequest struct {
	LoopRunID            string
	GateID               string
	CriterionID          string
	Attempt              int
	CorrelationID        string
	WorkspaceID          string
	Agent                string
	Model                string
	Rubric               string
	Contract             dsl.Contract
	NetworkParticipation *participation.Spec
}

// JudgeResponse is the raw judge response payload.
type JudgeResponse struct {
	Raw            string
	TokensUsed     int64
	TokensReported bool
}

// JudgeUsageReporter receives one aggregate cumulative token total for a gate evaluation.
type JudgeUsageReporter interface {
	ReportJudgeTokensUsed(int64)
}

// JudgeUsageReporterFunc adapts a function to JudgeUsageReporter.
type JudgeUsageReporterFunc func(int64)

// ReportJudgeTokensUsed implements JudgeUsageReporter.
func (f JudgeUsageReporterFunc) ReportJudgeTokensUsed(tokens int64) {
	if f != nil {
		f(tokens)
	}
}

// ToolCaller is the tools/call subset used by extension gate checks.
type ToolCaller interface {
	Call(ctx context.Context, scope tools.Scope, req tools.CallRequest) (tools.ToolResult, error)
}
