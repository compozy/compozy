package tools

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	// MaxClarifyChoices bounds model-authored choices before a question reaches a user.
	MaxClarifyChoices = 4
)

var (
	// ErrClarifyInvalid reports an invalid question or answer supplied at a public boundary.
	ErrClarifyInvalid = errors.New("tools: invalid clarification")
	// ErrClarifyPending reports that a session already owns an unresolved question.
	ErrClarifyPending = errors.New("tools: clarification already pending")
	// ErrClarifyNotFound reports that no live request matched the session and request ID.
	ErrClarifyNotFound = errors.New("tools: clarification not found")
	// ErrClarifyCanceled reports session-stop, caller-cancel, or daemon-shutdown cancellation.
	ErrClarifyCanceled = errors.New("tools: clarification canceled")
	// ErrClarifyClosed reports an ask attempted after broker shutdown began.
	ErrClarifyClosed = errors.New("tools: clarification broker closed")
)

// ClarifyStatus identifies one durable clarification lifecycle transition.
type ClarifyStatus string

const (
	ClarifyStatusPending  ClarifyStatus = "pending"
	ClarifyStatusResolved ClarifyStatus = "resolved"
	// ClarifyStatusTimedOut is no longer produced: clarifications wait until
	// answered. The status stays so durable history written before the change
	// still validates and projects.
	ClarifyStatusTimedOut ClarifyStatus = "timed_out"
	ClarifyStatusCanceled ClarifyStatus = "canceled"
)

// ClarifyAnswerSource records which input family produced a resolved answer.
type ClarifyAnswerSource string

const (
	// ClarifyAnswerSourceChoice marks an answer selected by choice_index.
	ClarifyAnswerSourceChoice ClarifyAnswerSource = "choice"
	// ClarifyAnswerSourceCustom marks free text: a question without choices, or
	// the "other" answer on a question with choices.
	ClarifyAnswerSourceCustom ClarifyAnswerSource = "custom"
)

// ClarifyQuestion is the model-authored request accepted by the broker.
type ClarifyQuestion struct {
	Question string   `json:"question"`
	Choices  []string `json:"choices,omitempty"`
}

// Normalize validates and isolates one clarification question.
func (q ClarifyQuestion) Normalize() (ClarifyQuestion, error) {
	normalized := ClarifyQuestion{Question: strings.TrimSpace(q.Question)}
	if normalized.Question == "" {
		return ClarifyQuestion{}, fmt.Errorf("%w: question is required", ErrClarifyInvalid)
	}
	if len(q.Choices) > MaxClarifyChoices {
		return ClarifyQuestion{}, fmt.Errorf(
			"%w: choices must contain at most %d items: %d",
			ErrClarifyInvalid,
			MaxClarifyChoices,
			len(q.Choices),
		)
	}
	if len(q.Choices) == 0 {
		return normalized, nil
	}
	normalized.Choices = make([]string, 0, len(q.Choices))
	for index, raw := range q.Choices {
		choice := strings.TrimSpace(raw)
		if choice == "" {
			return ClarifyQuestion{}, fmt.Errorf("%w: choices[%d] is required", ErrClarifyInvalid, index)
		}
		if slices.Contains(normalized.Choices, choice) {
			return ClarifyQuestion{}, fmt.Errorf(
				"%w: choices[%d] duplicates %q",
				ErrClarifyInvalid,
				index,
				choice,
			)
		}
		normalized.Choices = append(normalized.Choices, choice)
	}
	return normalized, nil
}

// ClarifyAnswer is the frozen answer returned to native and extension callers.
// Source is additive: answers persisted before it existed decode with an empty
// source, and Fallback is never set by a live wait any more.
type ClarifyAnswer struct {
	Choice   *int                `json:"choice"`
	Text     string              `json:"text"`
	Fallback bool                `json:"fallback"`
	Source   ClarifyAnswerSource `json:"source,omitempty"`
}

// Resolution returns the operator-visible value represented by a normalized answer.
func (a ClarifyAnswer) Resolution(question ClarifyQuestion) string {
	if a.Choice != nil && *a.Choice >= 0 && *a.Choice < len(question.Choices) {
		return strings.TrimSpace(question.Choices[*a.Choice])
	}
	return strings.TrimSpace(a.Text)
}

// ClarifyAnswerResult reports the canonical outcome of one answer mutation.
type ClarifyAnswerResult struct {
	ClarifyAnswer
	Outcome        string `json:"outcome"`
	InteractionID  string `json:"interaction_id,omitempty"`
	RequestID      string `json:"request_id"`
	ResolvedAnswer string `json:"resolved_answer,omitempty"`
}

// ClarifyAnswerRequest is the public answer mutation payload. Text is the
// free-text answer: the only answer on a question without choices, and the
// "other" answer on a question with choices. It is exclusive with ChoiceIndex.
type ClarifyAnswerRequest struct {
	ChoiceIndex *int   `json:"choice_index,omitzero"`
	Text        string `json:"text,omitempty"`
}

// Normalize validates an answer against the original question.
func (r ClarifyAnswerRequest) Normalize(question ClarifyQuestion) (ClarifyAnswer, error) {
	hasChoice := r.ChoiceIndex != nil
	text := strings.TrimSpace(r.Text)
	hasText := text != ""
	if hasChoice == hasText {
		return ClarifyAnswer{}, fmt.Errorf(
			"%w: answer requires exactly one of choice_index or text",
			ErrClarifyInvalid,
		)
	}
	if hasText && slices.Contains(question.Choices, text) {
		// A custom answer equal to a choice label would be indistinguishable from
		// that choice once the resolution is stored as a plain string.
		return ClarifyAnswer{}, fmt.Errorf(
			"%w: text repeats a choice; answer with choice_index instead",
			ErrClarifyInvalid,
		)
	}
	if hasChoice {
		index := *r.ChoiceIndex
		if len(question.Choices) == 0 {
			return ClarifyAnswer{}, fmt.Errorf(
				"%w: free-text question does not accept choice_index",
				ErrClarifyInvalid,
			)
		}
		if index < 0 || index >= len(question.Choices) {
			return ClarifyAnswer{}, fmt.Errorf(
				"%w: choice_index must be between 0 and %d: %d",
				ErrClarifyInvalid,
				len(question.Choices)-1,
				index,
			)
		}
		return ClarifyAnswer{Choice: new(index), Source: ClarifyAnswerSourceChoice}, nil
	}
	return ClarifyAnswer{Text: text, Source: ClarifyAnswerSourceCustom}, nil
}

// ClarifyPending is the complete live projection for one unresolved question.
type ClarifyPending struct {
	RequestID   string    `json:"request_id"`
	WorkspaceID string    `json:"workspace_id"`
	SessionID   string    `json:"session_id"`
	AgentName   string    `json:"agent_name"`
	Question    string    `json:"question"`
	Choices     []string  `json:"choices,omitempty"`
	AskedAt     time.Time `json:"asked_at"`
	// Deadline is always zero: a pending clarification waits until answered.
	// The field stays for wire compatibility and is projected as null.
	Deadline time.Time `json:"deadline"`
}

// Clone returns an isolated pending projection.
func (p ClarifyPending) Clone() ClarifyPending {
	cloned := p
	cloned.Choices = append([]string(nil), p.Choices...)
	return cloned
}

// ClarifyEvent is the durable session/SSE lifecycle payload.
type ClarifyEvent struct {
	Status     ClarifyStatus  `json:"status"`
	Request    ClarifyPending `json:"request"`
	Answer     *ClarifyAnswer `json:"answer,omitzero"`
	ResolvedBy string         `json:"resolved_by,omitempty"`
	At         time.Time      `json:"at"`
}

// Validate ensures lifecycle evidence cannot represent an impossible transition.
func (e ClarifyEvent) Validate() error {
	if err := e.validateIdentity(); err != nil {
		return err
	}
	switch e.Status {
	case ClarifyStatusPending:
		if e.Answer != nil {
			return errors.New("tools: pending clarification event cannot carry an answer")
		}
	case ClarifyStatusResolved:
		if e.Answer == nil || e.Answer.Fallback {
			return errors.New("tools: resolved clarification event requires a non-fallback answer")
		}
		if err := e.Answer.validateSource(); err != nil {
			return err
		}
	case ClarifyStatusTimedOut:
		if e.Answer == nil || e.Answer.Choice != nil || e.Answer.Text != "" || !e.Answer.Fallback {
			return errors.New("tools: timed-out clarification event requires the exact fallback sentinel")
		}
		if e.Request.Deadline.IsZero() {
			return errors.New("tools: timed-out clarification event requires a finite deadline")
		}
	case ClarifyStatusCanceled:
		if e.Answer != nil {
			return errors.New("tools: canceled clarification event cannot carry an answer")
		}
	default:
		return fmt.Errorf("tools: unsupported clarification status %q", e.Status)
	}
	return nil
}

func (e ClarifyEvent) validateIdentity() error {
	if strings.TrimSpace(e.Request.RequestID) == "" || strings.TrimSpace(e.Request.SessionID) == "" {
		return errors.New("tools: clarification event request and session IDs are required")
	}
	if strings.TrimSpace(e.Request.WorkspaceID) == "" || strings.TrimSpace(e.Request.AgentName) == "" {
		return errors.New("tools: clarification event workspace and agent are required")
	}
	if e.At.IsZero() || e.Request.AskedAt.IsZero() {
		return errors.New("tools: clarification event timestamps are required")
	}
	return nil
}

// validateSource checks a resolved answer's recorded source against its payload.
// Legacy answers predate Source and carry an empty value, which stays valid.
func (a ClarifyAnswer) validateSource() error {
	switch a.Source {
	case "":
		return nil
	case ClarifyAnswerSourceChoice:
		if a.Choice == nil || a.Text != "" {
			return errors.New("tools: choice clarification answer requires only a choice")
		}
		return nil
	case ClarifyAnswerSourceCustom:
		if a.Choice != nil || a.Text == "" {
			return errors.New("tools: custom clarification answer requires only text")
		}
		return nil
	default:
		return fmt.Errorf("tools: unsupported clarification answer source %q", a.Source)
	}
}

// ClarifyBroker owns the boot-scoped live clarification projection.
type ClarifyBroker interface {
	Ask(context.Context, Scope, ClarifyQuestion) (ClarifyAnswer, error)
	Pending(context.Context, Scope) ([]ClarifyPending, error)
	Answer(context.Context, Scope, string, ClarifyAnswerRequest) (ClarifyAnswerResult, error)
}
