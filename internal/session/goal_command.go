package session

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"
)

const maxGoalCommandCodePoints = 4000

// GoalDispatchKind selects a structured response or ordinary prompt continuation.
type GoalDispatchKind string

const (
	GoalDispatchRespond GoalDispatchKind = "respond"
	GoalDispatchPrompt  GoalDispatchKind = "prompt"
)

const (
	GoalCommandVerbSet     = "set"
	GoalCommandVerbReplace = "replace"
	GoalCommandVerbStatus  = "status"
	GoalCommandVerbPause   = "pause"
	GoalCommandVerbResume  = "resume"
	GoalCommandVerbClear   = "clear"
	GoalCommandVerbDraft   = "draft"
)

// GoalCommandOutcome is the session-domain structured Goal result vocabulary.
type GoalCommandOutcome string

const (
	GoalOutcomeStarted  GoalCommandOutcome = "started"
	GoalOutcomeReplaced GoalCommandOutcome = "replaced"
	GoalOutcomeStatus   GoalCommandOutcome = "status"
	GoalOutcomePaused   GoalCommandOutcome = "paused"
	GoalOutcomeResumed  GoalCommandOutcome = "resumed"
	GoalOutcomeCleared  GoalCommandOutcome = "cleared"
	GoalOutcomeError    GoalCommandOutcome = "error"
)

const (
	GoalReasonNotActive           GoalReasonCode = "goal_not_active"
	GoalReasonReplaceRequired     GoalReasonCode = "goal_replace_required"
	GoalReasonReplaceStale        GoalReasonCode = "goal_replace_stale"
	GoalReasonObjectiveRequired   GoalReasonCode = "goal_objective_required"
	GoalReasonObjectiveTooLarge   GoalReasonCode = "goal_objective_too_large"
	GoalReasonContractClauseEmpty GoalReasonCode = "goal_contract_clause_empty"
	GoalReasonCommandInvalid      GoalReasonCode = "goal_command_invalid"
	GoalReasonDraftRequiresIdle   GoalReasonCode = "goal_draft_requires_idle"
)

// PromptCaller is the authenticated operator identity admitted by an external prompt ingress.
type PromptCaller struct {
	Kind   string
	ID     string
	Source string
}

// Validate rejects incomplete caller provenance.
func (c PromptCaller) Validate() error {
	if strings.TrimSpace(c.Kind) == "" || strings.TrimSpace(c.ID) == "" || strings.TrimSpace(c.Source) == "" {
		return errors.New("session: prompt caller identity is incomplete")
	}
	return nil
}

// GoalCommand is one closed `/goal` command parsed at authenticated operator ingress.
type GoalCommand struct {
	Verb          string
	Objective     string
	ExpectedRunID string
}

// GoalObjectiveContract is the line-oriented free-form objective subset.
type GoalObjectiveContract struct {
	Objective   string
	Verify      []string
	Constraints []string
}

// GoalDispatchDecision returns either structured JSON or an ordinary streamed prompt rewrite.
type GoalDispatchDecision struct {
	Kind             GoalDispatchKind
	Result           *GoalCommandResult
	RewrittenMessage string
	BypassGoalParse  bool
	BusyPolicy       string
	BusyReason       GoalReasonCode
}

// GoalCommandResult is the session-domain structured command envelope.
type GoalCommandResult struct {
	Outcome       GoalCommandOutcome
	ReasonCode    *GoalReasonCode
	Snapshot      *GoalSnapshot
	ReplacedRunID *string
}

// GoalCommandHandler is the daemon-owned command execution boundary.
type GoalCommandHandler interface {
	Handle(
		context.Context,
		string,
		string,
		PromptCaller,
		GoalCommand,
	) (GoalDispatchDecision, error)
}

// GoalCommandHandlerFunc adapts a function to GoalCommandHandler.
type GoalCommandHandlerFunc func(
	context.Context,
	string,
	string,
	PromptCaller,
	GoalCommand,
) (GoalDispatchDecision, error)

// Handle implements GoalCommandHandler.
func (f GoalCommandHandlerFunc) Handle(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	caller PromptCaller,
	command GoalCommand,
) (GoalDispatchDecision, error) {
	decision, err := f(ctx, workspaceID, sessionID, caller, command)
	if err != nil {
		return GoalDispatchDecision{}, fmt.Errorf(
			"session: handle Goal command %q for session %q: %w",
			command.Verb,
			sessionID,
			err,
		)
	}
	return decision, nil
}

// GoalCommandError carries one deterministic parser reason.
type GoalCommandError struct {
	Code GoalReasonCode
	Err  error
}

func (e *GoalCommandError) Error() string {
	if e == nil || e.Err == nil {
		return "session: Goal command failed"
	}
	return e.Err.Error()
}

func (e *GoalCommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// ParseGoalCommand recognizes the exact leading `/goal` namespace and closed reserved verbs.
func ParseGoalCommand(message string) (GoalCommand, bool, error) {
	normalized := normalizeGoalCommandText(message)
	if !strings.HasPrefix(normalized, "/goal") {
		return GoalCommand{}, false, nil
	}
	if len(normalized) > len("/goal") {
		next := normalized[len("/goal")]
		if next != ' ' && next != '\t' && next != '\n' {
			return GoalCommand{}, false, nil
		}
	}
	payload := strings.TrimSpace(normalized[len("/goal"):])
	if payload == "" {
		return GoalCommand{}, true, goalCommandError(GoalReasonObjectiveRequired, "Goal objective is required")
	}
	verb, remainder := splitGoalCommandToken(payload)
	switch verb {
	case GoalCommandVerbStatus, GoalCommandVerbPause, GoalCommandVerbResume, GoalCommandVerbClear:
		if remainder != "" {
			return GoalCommand{}, true, goalCommandError(
				GoalReasonCommandInvalid,
				"Goal command has unexpected arguments",
			)
		}
		return GoalCommand{Verb: verb}, true, nil
	case GoalCommandVerbReplace:
		expectedRunID, objective := splitGoalCommandToken(remainder)
		if expectedRunID == "" || objective == "" {
			return GoalCommand{}, true, goalCommandError(
				GoalReasonCommandInvalid,
				"Goal replace requires an expected Run ID and objective",
			)
		}
		if err := validateGoalCommandPayload(objective); err != nil {
			return GoalCommand{}, true, err
		}
		return GoalCommand{Verb: verb, ExpectedRunID: expectedRunID, Objective: objective}, true, nil
	case GoalCommandVerbDraft:
		if remainder == "" {
			return GoalCommand{}, true, goalCommandError(GoalReasonObjectiveRequired, "Goal draft text is required")
		}
		if err := validateGoalCommandPayload(remainder); err != nil {
			return GoalCommand{}, true, err
		}
		return GoalCommand{Verb: verb, Objective: remainder}, true, nil
	case GoalCommandVerbSet:
		if remainder == "" {
			return GoalCommand{}, true, goalCommandError(
				GoalReasonObjectiveRequired,
				"Goal objective is required",
			)
		}
		if err := validateGoalCommandPayload(remainder); err != nil {
			return GoalCommand{}, true, err
		}
		return GoalCommand{Verb: verb, Objective: remainder}, true, nil
	default:
		if err := validateGoalCommandPayload(payload); err != nil {
			return GoalCommand{}, true, err
		}
		return GoalCommand{Verb: GoalCommandVerbSet, Objective: payload}, true, nil
	}
}

// ParseGoalObjective extracts exact lowercase verify/constraints clauses without creating commands.
func ParseGoalObjective(raw string) (GoalObjectiveContract, error) {
	normalized := normalizeGoalCommandText(raw)
	contract := GoalObjectiveContract{Verify: []string{}, Constraints: []string{}}
	objective := make([]string, 0)
	for line := range strings.SplitSeq(normalized, "\n") {
		trimmed := strings.TrimSpace(line)
		if remainder, ok := strings.CutPrefix(trimmed, "verify:"); ok {
			value := strings.TrimSpace(remainder)
			if value == "" {
				return GoalObjectiveContract{}, goalCommandError(
					GoalReasonContractClauseEmpty,
					"Goal verify clause is empty",
				)
			}
			contract.Verify = append(contract.Verify, value)
			continue
		}
		if remainder, ok := strings.CutPrefix(trimmed, "constraints:"); ok {
			value := strings.TrimSpace(remainder)
			if value == "" {
				return GoalObjectiveContract{}, goalCommandError(
					GoalReasonContractClauseEmpty,
					"Goal constraints clause is empty",
				)
			}
			contract.Constraints = append(contract.Constraints, value)
			continue
		}
		if trimmed != "" {
			objective = append(objective, trimmed)
		}
	}
	contract.Objective = strings.Join(objective, "\n")
	if contract.Objective == "" {
		return GoalObjectiveContract{}, goalCommandError(GoalReasonObjectiveRequired, "Goal objective is required")
	}
	return contract, nil
}

func normalizeGoalCommandText(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(value, "\r\n", "\n"), "\r", "\n")
}

func splitGoalCommandToken(value string) (string, string) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", ""
	}
	for index, character := range trimmed {
		if character == ' ' || character == '\t' || character == '\n' {
			return trimmed[:index], strings.TrimSpace(trimmed[index:])
		}
	}
	return trimmed, ""
}

func validateGoalCommandPayload(value string) error {
	if utf8.RuneCountInString(strings.TrimSpace(value)) > maxGoalCommandCodePoints {
		return goalCommandError(GoalReasonObjectiveTooLarge, "Goal objective exceeds 4,000 code points")
	}
	return nil
}

func goalCommandError(code GoalReasonCode, message string) error {
	return &GoalCommandError{Code: code, Err: fmt.Errorf("session: %s", message)}
}
