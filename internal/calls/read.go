package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const (
	// DefaultReadLimit is the page size used when callers omit a limit.
	DefaultReadLimit = 50
	// MaxReadLimit is the largest public call or message page.
	MaxReadLimit = 200
)

// NormalizeReadScope infers and validates the exact global or workspace call boundary.
func NormalizeReadScope(scope Scope, workspaceID string) (Scope, string, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	scope = Scope(strings.TrimSpace(string(scope)))
	if scope == "" {
		if workspaceID == "" {
			scope = ScopeGlobal
		} else {
			scope = ScopeWorkspace
		}
	}
	if scope != ScopeGlobal && scope != ScopeWorkspace {
		return "", "", fmt.Errorf("scope must be global or workspace")
	}
	if scope == ScopeGlobal && workspaceID != "" || scope == ScopeWorkspace && workspaceID == "" {
		return "", "", fmt.Errorf("scope and workspace_id do not match")
	}
	return scope, workspaceID, nil
}

// NormalizeReadQuery trims, infers, and validates a public call read boundary.
func NormalizeReadQuery(query CallReadQuery) (CallReadQuery, error) {
	query.ReadScope.ProfileID = strings.TrimSpace(query.ReadScope.ProfileID)
	if err := query.ReadScope.Validate(); err != nil {
		return CallReadQuery{}, newError(CodeValidation, "read scope must select one profile or all profiles", err)
	}
	scope, workspaceID, err := NormalizeReadScope(query.Scope, query.WorkspaceID)
	if err != nil {
		return CallReadQuery{}, newError(CodeValidation, "invalid call read scope", err)
	}
	query.Scope = scope
	query.WorkspaceID = workspaceID
	return query, nil
}

// CallReadQuery selects one profile or the explicit all-profile aggregate call population.
type CallReadQuery struct {
	ReadScope   store.ReadScope
	Scope       Scope
	WorkspaceID string
}

// CallListQuery selects a stable page of call records.
type CallListQuery struct {
	CallReadQuery
	State          []State
	Attention      bool
	Caller         string
	ChildSessionID string
	RootSessionID  string
	Agent          string
	Cursor         string
	Limit          int
}

// CallPage is a counted, cursor-backed page in durable creation order.
type CallPage struct {
	Items      []CallRecord
	NextCursor string
	Total      int
}

// MessageListQuery selects a stable page of lineage mailbox records.
type MessageListQuery struct {
	CallReadQuery
	SessionID string
	Cursor    string
	Limit     int
}

// MessagePage is an uncounted, cursor-backed mailbox page.
type MessagePage struct {
	Items      []MessageRecord
	NextCursor string
}

// ResultPayload contains the exact stored bytes for one completed call.
type ResultPayload struct {
	CallID string
	Bytes  []byte
}

// PromptPayload contains the exact authored prompt for one caller-owned call.
type PromptPayload struct {
	CallID string
	Text   string
}

// List returns one profile-aware page with filters applied before the cut.
func (s *Service) List(ctx context.Context, query CallListQuery) (CallPage, error) {
	reader, err := s.callListStore()
	if err != nil {
		return CallPage{}, err
	}
	query = normalizeCallListQuery(query)
	query.CallReadQuery, err = NormalizeReadQuery(query.CallReadQuery)
	if err != nil {
		return CallPage{}, err
	}
	return reader.ListCalls(ctx, query)
}

// GetRead returns one call through the same exact-or-aggregate read boundary as List.
func (s *Service) GetRead(ctx context.Context, query CallReadQuery, callID string) (CallRecord, error) {
	reader, err := s.callReadStore()
	if err != nil {
		return CallRecord{}, err
	}
	query, err = NormalizeReadQuery(query)
	if err != nil {
		return CallRecord{}, err
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return CallRecord{}, newError(CodeValidation, "call_id is required", nil)
	}
	return reader.GetCallRead(ctx, query, callID)
}

// Result returns the whole stored result and refuses calls that have not produced one.
func (s *Service) Result(ctx context.Context, query CallReadQuery, callID string) (ResultPayload, error) {
	record, err := s.GetRead(ctx, query, callID)
	if err != nil {
		return ResultPayload{}, err
	}
	if record.State != StateCompleted || strings.TrimSpace(record.ResultRef) == "" {
		return ResultPayload{}, newError(
			CodeNotSettled,
			fmt.Sprintf("call %q is %s; await completion before fetching the result", record.CallID, record.State),
			nil,
		)
	}
	payload, err := s.readPayload(ctx, record.WorkspaceID, record.ResultRef)
	if err != nil {
		return ResultPayload{}, err
	}
	return ResultPayload{CallID: record.CallID, Bytes: append([]byte(nil), payload...)}, nil
}

// Prompt returns the exact authored prompt without exposing its storage reference.
func (s *Service) Prompt(ctx context.Context, query CallReadQuery, callID string) (PromptPayload, error) {
	record, err := s.GetRead(ctx, query, callID)
	if err != nil {
		return PromptPayload{}, err
	}
	payload, err := s.readPayload(ctx, record.WorkspaceID, record.PromptRef)
	if err != nil {
		return PromptPayload{}, err
	}
	return PromptPayload{CallID: record.CallID, Text: string(payload)}, nil
}

// Superseded returns preserved late-result evidence without exposing its storage reference.
func (s *Service) Superseded(ctx context.Context, query CallReadQuery, callID string) (ResultPayload, error) {
	record, err := s.GetRead(ctx, query, callID)
	if err != nil {
		return ResultPayload{}, err
	}
	if strings.TrimSpace(record.SupersededRef) == "" {
		return ResultPayload{}, newError(CodeNotSettled, "call has no superseded result evidence", nil)
	}
	payload, err := s.readPayload(ctx, record.WorkspaceID, record.SupersededRef)
	if err != nil {
		return ResultPayload{}, err
	}
	return ResultPayload{CallID: record.CallID, Bytes: append([]byte(nil), payload...)}, nil
}

// ListMessages returns one profile-aware mailbox page.
func (s *Service) ListMessages(ctx context.Context, query MessageListQuery) (MessagePage, error) {
	reader, err := s.messageReadStore()
	if err != nil {
		return MessagePage{}, err
	}
	query.CallReadQuery, err = NormalizeReadQuery(query.CallReadQuery)
	if err != nil {
		return MessagePage{}, err
	}
	query.SessionID = strings.TrimSpace(query.SessionID)
	query.Cursor = strings.TrimSpace(query.Cursor)
	query.Limit = normalizeReadLimit(query.Limit)
	return reader.ListMessages(ctx, query)
}

func normalizeCallListQuery(query CallListQuery) CallListQuery {
	query.Caller = strings.TrimSpace(query.Caller)
	query.ChildSessionID = strings.TrimSpace(query.ChildSessionID)
	query.RootSessionID = strings.TrimSpace(query.RootSessionID)
	query.Agent = strings.TrimSpace(query.Agent)
	query.Cursor = strings.TrimSpace(query.Cursor)
	query.Limit = normalizeReadLimit(query.Limit)
	return query
}

func normalizeReadLimit(limit int) int {
	if limit <= 0 {
		return DefaultReadLimit
	}
	if limit > MaxReadLimit {
		return MaxReadLimit
	}
	return limit
}

func (s *Service) callListStore() (CallListStore, error) {
	reader, ok := s.store.(CallListStore)
	if !ok {
		return nil, errors.New("calls: store does not implement public call pages")
	}
	return reader, nil
}

func (s *Service) callReadStore() (CallReadStore, error) {
	reader, ok := s.store.(CallReadStore)
	if !ok {
		return nil, errors.New("calls: store does not implement public call detail")
	}
	return reader, nil
}

func (s *Service) messageReadStore() (MessageReadStore, error) {
	reader, ok := s.store.(MessageReadStore)
	if !ok {
		return nil, errors.New("calls: store does not implement public mailbox pages")
	}
	return reader, nil
}

func (s *Service) payloadStore() (PayloadStore, error) {
	reader, ok := s.store.(PayloadStore)
	if !ok {
		return nil, errors.New("calls: store does not implement payload reads")
	}
	return reader, nil
}

func (s *Service) readPayload(ctx context.Context, workspaceID, ref string) ([]byte, error) {
	reader, err := s.payloadStore()
	if err != nil {
		return nil, err
	}
	return reader.GetCallPayload(ctx, strings.TrimSpace(workspaceID), strings.TrimSpace(ref))
}
