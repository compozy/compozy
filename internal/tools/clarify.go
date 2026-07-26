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
	ClarifyStatusTimedOut ClarifyStatus = "timed_out"
	ClarifyStatusCanceled ClarifyStatus = "canceled"
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
type ClarifyAnswer struct {
	Choice   *int   `json:"choice"`
	Text     string `json:"text"`
	Fallback bool   `json:"fallback"`
}

// ClarifyAnswerRequest is the public answer mutation payload.
type ClarifyAnswerRequest struct {
	ChoiceIndex *int   `json:"choice_index,omitempty"`
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
		choice := index
		return ClarifyAnswer{Choice: &choice}, nil
	}
	return ClarifyAnswer{Text: text}, nil
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
	Deadline    time.Time `json:"deadline"`
}

// Clone returns an isolated pending projection.
func (p ClarifyPending) Clone() ClarifyPending {
	cloned := p
	cloned.Choices = append([]string(nil), p.Choices...)
	return cloned
}

// ClarifyEvent is the durable session/SSE lifecycle payload.
type ClarifyEvent struct {
	Status  ClarifyStatus  `json:"status"`
	Request ClarifyPending `json:"request"`
	Answer  *ClarifyAnswer `json:"answer,omitempty"`
	At      time.Time      `json:"at"`
}

// Validate ensures lifecycle evidence cannot represent an impossible transition.
func (e ClarifyEvent) Validate() error {
	if strings.TrimSpace(e.Request.RequestID) == "" || strings.TrimSpace(e.Request.SessionID) == "" {
		return errors.New("tools: clarification event request and session IDs are required")
	}
	if strings.TrimSpace(e.Request.WorkspaceID) == "" || strings.TrimSpace(e.Request.AgentName) == "" {
		return errors.New("tools: clarification event workspace and agent are required")
	}
	if e.At.IsZero() || e.Request.AskedAt.IsZero() || e.Request.Deadline.IsZero() {
		return errors.New("tools: clarification event timestamps are required")
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
	case ClarifyStatusTimedOut:
		if e.Answer == nil || e.Answer.Choice != nil || e.Answer.Text != "" || !e.Answer.Fallback {
			return errors.New("tools: timed-out clarification event requires the exact fallback sentinel")
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

// ClarifyBroker owns the boot-scoped live clarification projection.
type ClarifyBroker interface {
	Ask(context.Context, Scope, ClarifyQuestion) (ClarifyAnswer, error)
	Pending(context.Context, Scope) ([]ClarifyPending, error)
	Answer(context.Context, Scope, string, ClarifyAnswerRequest) (ClarifyAnswer, error)
}
