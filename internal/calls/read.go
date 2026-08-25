package calls

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/store"
)

const (
	DefaultReadLimit = 50
	MaxReadLimit     = 200
)

// CallReadQuery selects one profile-owned call population.
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
	if err := validateReadQuery(query.CallReadQuery); err != nil {
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
	query = normalizeCallReadQuery(query)
	if err := validateReadQuery(query); err != nil {
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
	mailbox, err := s.payloadStore()
	if err != nil {
		return ResultPayload{}, err
	}
	payload, err := mailbox.GetCallPayload(ctx, record.WorkspaceID, record.ResultRef)
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
	mailbox, err := s.payloadStore()
	if err != nil {
		return PromptPayload{}, err
	}
	payload, err := mailbox.GetCallPayload(ctx, record.WorkspaceID, record.PromptRef)
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
	mailbox, err := s.payloadStore()
	if err != nil {
		return ResultPayload{}, err
	}
	payload, err := mailbox.GetCallPayload(ctx, record.WorkspaceID, record.SupersededRef)
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
	query.CallReadQuery = normalizeCallReadQuery(query.CallReadQuery)
	query.SessionID = strings.TrimSpace(query.SessionID)
	query.Cursor = strings.TrimSpace(query.Cursor)
	query.Limit = normalizeReadLimit(query.Limit)
	if err := validateReadQuery(query.CallReadQuery); err != nil {
		return MessagePage{}, err
	}
	return reader.ListMessages(ctx, query)
}

func normalizeCallListQuery(query CallListQuery) CallListQuery {
	query.CallReadQuery = normalizeCallReadQuery(query.CallReadQuery)
	query.Caller = strings.TrimSpace(query.Caller)
	query.ChildSessionID = strings.TrimSpace(query.ChildSessionID)
	query.RootSessionID = strings.TrimSpace(query.RootSessionID)
	query.Agent = strings.TrimSpace(query.Agent)
	query.Cursor = strings.TrimSpace(query.Cursor)
	query.Limit = normalizeReadLimit(query.Limit)
	return query
}

func normalizeCallReadQuery(query CallReadQuery) CallReadQuery {
	query.ReadScope.ProfileID = strings.TrimSpace(query.ReadScope.ProfileID)
	query.WorkspaceID = strings.TrimSpace(query.WorkspaceID)
	if query.Scope == "" && query.WorkspaceID != "" {
		query.Scope = ScopeWorkspace
	}
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

func validateReadQuery(query CallReadQuery) error {
	if err := query.ReadScope.Validate(); err != nil {
		return newError(CodeValidation, "read scope must select one profile or all profiles", err)
	}
	switch query.Scope {
	case "":
		if query.WorkspaceID != "" {
			return newError(CodeValidation, "workspace_id requires workspace scope", nil)
		}
	case ScopeGlobal:
		if query.WorkspaceID != "" {
			return newError(CodeValidation, "global scope requires an empty workspace_id", nil)
		}
	case ScopeWorkspace:
		if query.WorkspaceID == "" {
			return newError(CodeValidation, "workspace scope requires workspace_id", nil)
		}
	default:
		return newError(CodeValidation, fmt.Sprintf("unsupported scope %q", query.Scope), nil)
	}
	return nil
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
