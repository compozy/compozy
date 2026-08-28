package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	terminalpkg "github.com/compozy/compozy/internal/terminal"
)

type TerminalExecRequest struct {
	Command string                  `json:"command"`
	Args    []string                `json:"args,omitempty"`
	Cwd     string                  `json:"cwd,omitempty"`
	Env     map[string]string       `json:"env,omitempty"`
	YieldMs int                     `json:"yield_ms,omitempty"`
	Visible bool                    `json:"visible,omitempty"`
	Output  terminalpkg.OutputShape `json:"output"`
}

type TerminalReadOptions struct {
	View     string
	MaxBytes int
	SinceSeq uint64
	FromLine int
	ToLine   int
	Grep     string
}

type TerminalInputRequestQuery struct {
	TerminalID string
}

type TerminalInputRequests struct {
	Pending  []terminalpkg.PendingInputRequest  `json:"pending"`
	Resolved []terminalpkg.ResolvedInputRequest `json:"resolved"`
}

type TerminalJournalQuery struct {
	Actor      string
	Since      string
	TerminalID string
	Failed     bool
	Limit      int
	Cursor     string
}

type TerminalAgentClient interface {
	TerminalClient
	ExecTerminal(context.Context, string, TerminalExecRequest) (terminalpkg.ExecResult, error)
	ReadTerminal(context.Context, string, string, TerminalReadOptions) (terminalpkg.ReadResult, error)
	SignalTerminal(context.Context, string, string, string) error
	ListTerminalInputRequests(
		context.Context,
		string,
		TerminalInputRequestQuery,
	) (TerminalInputRequests, error)
	AnswerTerminalInputRequest(context.Context, string, string, string, []byte) (int, bool, error)
	RejectTerminalInputRequest(context.Context, string, string, string, string) error
	QueryTerminalJournal(context.Context, string, TerminalJournalQuery) (terminalpkg.Page, error)
	ControlTerminalRecording(context.Context, string, string, string) (terminalpkg.RecordingRef, error)
}

var _ TerminalAgentClient = (*daemonClient)(nil)

func (c *daemonClient) ExecTerminal(
	ctx context.Context,
	workspace string,
	request TerminalExecRequest,
) (terminalpkg.ExecResult, error) {
	var response contract.TerminalExecResponse
	if err := c.doJSON(
		ctx, http.MethodPost, terminalClientPath(workspace)+"/exec", nil,
		terminalExecRequestContract(request), &response,
	); err != nil {
		return terminalpkg.ExecResult{}, err
	}
	result, err := terminalExecResultFromContract(response)
	if err != nil {
		return terminalpkg.ExecResult{}, fmt.Errorf("cli: decode terminal exec response: %w", err)
	}
	return result, nil
}

func (c *daemonClient) ReadTerminal(
	ctx context.Context,
	workspace string,
	id string,
	options TerminalReadOptions,
) (terminalpkg.ReadResult, error) {
	query := make(url.Values)
	query.Set("view", options.View)
	setTerminalPositiveInt(query, "max_bytes", options.MaxBytes)
	setTerminalPositiveUint64(query, "since_seq", options.SinceSeq)
	setTerminalPositiveInt(query, "from", options.FromLine)
	setTerminalPositiveInt(query, "to", options.ToLine)
	if options.Grep != "" {
		query.Set("grep", options.Grep)
	}
	var response contract.TerminalReadResponse
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/read"
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return terminalpkg.ReadResult{}, err
	}
	sequence, err := response.Seq.Uint64()
	if err != nil {
		return terminalpkg.ReadResult{}, fmt.Errorf("cli: decode terminal read sequence: %w", err)
	}
	spill, err := terminalSpillFromContractStrict(response.Spill)
	if err != nil {
		return terminalpkg.ReadResult{}, fmt.Errorf("cli: decode terminal read spill: %w", err)
	}
	return terminalpkg.ReadResult{
		Content: response.Content, Seq: sequence, Truncated: response.Truncated,
		Busy: response.Busy, Untrusted: response.Untrusted, Spill: spill,
	}, nil
}

func (c *daemonClient) SignalTerminal(ctx context.Context, workspace, id, signal string) error {
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/signal"
	if !slices.Contains(contract.TerminalSignalValues(), signal) {
		return fmt.Errorf("unsupported terminal signal %q", signal)
	}
	return c.doJSON(ctx, http.MethodPost, path, nil, contract.TerminalSignalRequest{
		Signal: contract.TerminalSignal(signal),
	}, nil)
}

func (c *daemonClient) ListTerminalInputRequests(
	ctx context.Context,
	workspace string,
	queryInput TerminalInputRequestQuery,
) (TerminalInputRequests, error) {
	query := make(url.Values)
	if queryInput.TerminalID != "" {
		query.Set("terminal_id", queryInput.TerminalID)
	}
	var response contract.TerminalInputRequestsResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		terminalClientPath(workspace)+"/input-requests",
		query,
		nil,
		&response,
	); err != nil {
		return TerminalInputRequests{}, err
	}
	requests, err := terminalInputRequestsFromContract(response)
	if err != nil {
		return TerminalInputRequests{}, fmt.Errorf("cli: decode terminal input response: %w", err)
	}
	return requests, nil
}

func (c *daemonClient) AnswerTerminalInputRequest(
	ctx context.Context,
	workspace, terminalID, requestID string,
	input []byte,
) (int, bool, error) {
	var response contract.TerminalInputAnswerResponse
	path := terminalInputRequestPath(workspace, terminalID, requestID) + "/answer"
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		contract.TerminalAnswerInputRequest{Input: string(input)},
		&response,
	); err != nil {
		return 0, false, err
	}
	return response.DeliveredBytes, response.Redacted, nil
}

func (c *daemonClient) RejectTerminalInputRequest(
	ctx context.Context,
	workspace, terminalID, requestID, reason string,
) error {
	path := terminalInputRequestPath(workspace, terminalID, requestID) + "/reject"
	return c.doJSON(ctx, http.MethodPost, path, nil, contract.TerminalRejectInputRequest{Reason: reason}, nil)
}

func (c *daemonClient) QueryTerminalJournal(
	ctx context.Context,
	workspace string,
	queryInput TerminalJournalQuery,
) (terminalpkg.Page, error) {
	query := make(url.Values)
	setTerminalString(query, "actor", queryInput.Actor)
	setTerminalString(query, "since", queryInput.Since)
	setTerminalString(query, "terminal_id", queryInput.TerminalID)
	setTerminalString(query, "cursor", queryInput.Cursor)
	setTerminalPositiveInt(query, "limit", queryInput.Limit)
	if queryInput.Failed {
		query.Set("failed", "true")
	}
	var response contract.TerminalJournalResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		terminalClientPath(workspace)+"/journal",
		query,
		nil,
		&response,
	); err != nil {
		return terminalpkg.Page{}, err
	}
	entries := make([]terminalpkg.CommandRow, 0, len(response.Entries))
	for _, row := range response.Entries {
		entries = append(entries, contract.TerminalCommandRowFromPayload(row))
	}
	next := ""
	if response.Next != nil {
		next = *response.Next
	}
	return terminalpkg.Page{Entries: entries, Next: next}, nil
}

func (c *daemonClient) ControlTerminalRecording(
	ctx context.Context,
	workspace, terminalID, action string,
) (terminalpkg.RecordingRef, error) {
	request, err := terminalRecordingRequest(action)
	if err != nil {
		return terminalpkg.RecordingRef{}, err
	}
	var response contract.TerminalRecordingResponse
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(terminalID)) + "/recording"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return terminalpkg.RecordingRef{}, err
	}
	recording, err := terminalRecordingFromContract(response.Recording)
	if err != nil {
		return terminalpkg.RecordingRef{}, fmt.Errorf("cli: decode terminal recording response: %w", err)
	}
	return recording, nil
}

func terminalInputRequestPath(workspace, terminalID, requestID string) string {
	return terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(terminalID)) +
		"/input-requests/" + url.PathEscape(strings.TrimSpace(requestID))
}

func setTerminalString(query url.Values, key, value string) {
	if value != "" {
		query.Set(key, value)
	}
}

func setTerminalPositiveInt(query url.Values, key string, value int) {
	if value > 0 {
		query.Set(key, strconv.Itoa(value))
	}
}

func setTerminalPositiveUint64(query url.Values, key string, value uint64) {
	if value > 0 {
		query.Set(key, strconv.FormatUint(value, 10))
	}
}
