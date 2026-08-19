package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/task"
)

const (
	// NodeWaitKindTimer parks a cell until its durable timestamp is due.
	NodeWaitKindTimer = "timer"
	// NodeWaitKindEvent parks a cell until a matching watch event is admitted.
	NodeWaitKindEvent = "event"
	// NodeWaitKindApprovalEscalation parks a gate while its escalation ladder remains active.
	NodeWaitKindApprovalEscalation = "approval_escalation"
	// NodeWaitKindRequest parks a human request until answer, expiry, or cancellation.
	NodeWaitKindRequest = "request"
)

// NodeWaitIntent persists one wait in the same transaction as its waiting cell.
type NodeWaitIntent struct {
	NodeID           NodeID          `json:"node_id"`
	ItemIndex        int             `json:"item_index,omitempty"`
	Kind             string          `json:"kind"`
	ResumeAt         *time.Time      `json:"resume_at,omitempty"`
	NextEscalationAt *time.Time      `json:"next_escalation_at,omitempty"`
	Expect           json.RawMessage `json:"expect,omitempty"`
	ClaimState       WaitClaimState  `json:"claim_state,omitempty"`
	ClaimedByKind    string          `json:"claimed_by_kind,omitempty"`
	ClaimedByID      string          `json:"claimed_by_id,omitempty"`
	ClaimedAt        *time.Time      `json:"claimed_at,omitempty"`
	AheadPayload     json.RawMessage `json:"ahead_payload,omitempty"`
	IssuedEpoch      int64           `json:"issued_epoch"`
	CreatedAt        time.Time       `json:"created_at"`
}

// WaitResumeMutation admits one payload into a durable wait.
type WaitResumeMutation struct {
	WorkspaceID       WorkspaceID
	RunID             RunID
	Generation        int
	NodeID            NodeID
	ItemIndex         int
	Payload           json.RawMessage
	ClaimedByKind     string
	ClaimedByID       string
	AdmissionAttempts int
	RequestedAt       time.Time
}

// WaitResumeResult reports the winning provenance and committed coordinator wake.
type WaitResumeResult struct {
	Wait        NodeWait
	Coordinator *task.Run
	Won         bool
}

type waitAheadRejectFence struct {
	Policy  string           `json:"policy"`
	Cursors map[string]int64 `json:"cursors"`
}

func encodeWaitAheadRejectFence(cursors map[string]int64) (json.RawMessage, error) {
	raw, err := json.Marshal(waitAheadRejectFence{Policy: "reject", Cursors: cloneInt64Map(cursors)})
	if err != nil {
		return nil, fmt.Errorf("loop: encode wait ahead-arrival fence: %w", err)
	}
	return raw, nil
}

// DecodeWaitAheadRejectCursors returns the stream fence captured when a reject-policy wait entered.
func DecodeWaitAheadRejectCursors(raw json.RawMessage) (map[string]int64, bool, error) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	var fence waitAheadRejectFence
	if err := json.Unmarshal(raw, &fence); err != nil {
		return nil, false, fmt.Errorf("%w: decode wait ahead-arrival fence: %v", ErrValidation, err)
	}
	if strings.TrimSpace(fence.Policy) != "reject" {
		return nil, false, nil
	}
	for stream, cursor := range fence.Cursors {
		if strings.TrimSpace(stream) == "" || cursor < 0 {
			return nil, false, fmt.Errorf("%w: wait ahead-arrival fence is invalid", ErrValidation)
		}
	}
	return cloneInt64Map(fence.Cursors), true, nil
}

// WaitAdmission owns the atomic wait claim and coordinator reservation.
type WaitAdmission interface {
	ResumeWait(context.Context, WaitResumeMutation) (WaitResumeResult, error)
}

func (i NodeWaitIntent) normalized() NodeWaitIntent {
	i.NodeID = NodeID(strings.TrimSpace(string(i.NodeID)))
	i.Kind = strings.TrimSpace(i.Kind)
	if i.ClaimState == "" {
		i.ClaimState = WaitClaimWaiting
	}
	i.ClaimedByKind = strings.TrimSpace(i.ClaimedByKind)
	i.ClaimedByID = strings.TrimSpace(i.ClaimedByID)
	if i.ResumeAt != nil {
		value := i.ResumeAt.UTC()
		i.ResumeAt = &value
	}
	if i.NextEscalationAt != nil {
		value := i.NextEscalationAt.UTC()
		i.NextEscalationAt = &value
	}
	if len(i.Expect) > 0 {
		i.Expect = append(json.RawMessage(nil), i.Expect...)
	}
	if i.ClaimedAt != nil {
		value := i.ClaimedAt.UTC()
		i.ClaimedAt = &value
	}
	if len(i.AheadPayload) > 0 {
		i.AheadPayload = append(json.RawMessage(nil), i.AheadPayload...)
	}
	if !i.CreatedAt.IsZero() {
		i.CreatedAt = i.CreatedAt.UTC()
	}
	return i
}

func (i NodeWaitIntent) validate() error {
	if i.NodeID == "" || i.ItemIndex < 0 || i.IssuedEpoch < 1 || i.CreatedAt.IsZero() {
		return fmt.Errorf("%w: node wait intent identity is incomplete", ErrValidation)
	}
	if !validNodeWaitKind(i.Kind) {
		return fmt.Errorf("%w: node wait kind is invalid: %q", ErrValidation, i.Kind)
	}
	if i.Kind == NodeWaitKindTimer && i.ResumeAt == nil {
		return fmt.Errorf("%w: timer wait requires resume_at", ErrValidation)
	}
	if len(i.Expect) > 0 && !json.Valid(i.Expect) {
		return fmt.Errorf("%w: node wait expect must be valid JSON", ErrValidation)
	}
	if i.ClaimState != WaitClaimWaiting && i.ClaimState != WaitClaimResumed {
		return fmt.Errorf("%w: node wait intent claim state is invalid", ErrValidation)
	}
	if i.ClaimState == WaitClaimResumed && !i.hasResumeProvenance() {
		return fmt.Errorf("%w: resumed node wait intent provenance is incomplete", ErrValidation)
	}
	return nil
}

func validNodeWaitKind(kind string) bool {
	return kind == NodeWaitKindTimer || kind == NodeWaitKindEvent ||
		kind == NodeWaitKindApprovalEscalation || kind == NodeWaitKindRequest
}

func (i NodeWaitIntent) hasResumeProvenance() bool {
	return i.ClaimedByKind != "" && i.ClaimedByID != "" && i.ClaimedAt != nil &&
		len(i.AheadPayload) > 0 && json.Valid(i.AheadPayload)
}

// Validate rejects wait admissions that cannot preserve identity and provenance.
func (m WaitResumeMutation) Validate() error {
	if strings.TrimSpace(string(m.WorkspaceID)) == "" || strings.TrimSpace(string(m.RunID)) == "" ||
		m.Generation < 1 || strings.TrimSpace(string(m.NodeID)) == "" || m.ItemIndex < 0 ||
		strings.TrimSpace(m.ClaimedByKind) == "" || strings.TrimSpace(m.ClaimedByID) == "" ||
		m.AdmissionAttempts < 1 || m.RequestedAt.IsZero() {
		return fmt.Errorf("%w: wait resume identity is incomplete", ErrValidation)
	}
	if len(m.Payload) == 0 || !json.Valid(m.Payload) {
		return fmt.Errorf("%w: wait resume payload must be valid JSON", ErrValidation)
	}
	return nil
}

func normalizeNodeWaitIntents(intents []NodeWaitIntent) ([]NodeWaitIntent, error) {
	if len(intents) == 0 {
		return intents, nil
	}
	result := make([]NodeWaitIntent, len(intents))
	for index, intent := range intents {
		normalized := intent.normalized()
		if err := normalized.validate(); err != nil {
			return nil, err
		}
		result[index] = normalized
	}
	return result, nil
}
