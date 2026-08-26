package cli

import (
	"context"
	"net/http"
	"net/url"
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
	) ([]terminalpkg.PendingInputRequest, error)
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
	var result terminalpkg.ExecResult
	if err := c.doJSON(ctx, http.MethodPost, terminalClientPath(workspace)+"/exec", nil, request, &result); err != nil {
		return terminalpkg.ExecResult{}, err
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
	var result terminalpkg.ReadResult
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/read"
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &result); err != nil {
		return terminalpkg.ReadResult{}, err
	}
	return result, nil
}

func (c *daemonClient) SignalTerminal(ctx context.Context, workspace, id, signal string) error {
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(id)) + "/signal"
	return c.doJSON(ctx, http.MethodPost, path, nil, map[string]string{"signal": signal}, nil)
}

func (c *daemonClient) ListTerminalInputRequests(
	ctx context.Context,
	workspace string,
	queryInput TerminalInputRequestQuery,
) ([]terminalpkg.PendingInputRequest, error) {
	query := make(url.Values)
	if queryInput.TerminalID != "" {
		query.Set("terminal_id", queryInput.TerminalID)
	}
	var response struct {
		Requests []terminalpkg.PendingInputRequest `json:"requests"`
	}
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		terminalClientPath(workspace)+"/input-requests",
		query,
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Requests, nil
}

func (c *daemonClient) AnswerTerminalInputRequest(
	ctx context.Context,
	workspace, terminalID, requestID string,
	input []byte,
) (int, bool, error) {
	var response struct {
		DeliveredBytes int  `json:"delivered_bytes"`
		Redacted       bool `json:"redacted"`
	}
	path := terminalInputRequestPath(workspace, terminalID, requestID) + "/answer"
	if err := c.doJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		map[string]string{"input": string(input)},
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
	return c.doJSON(ctx, http.MethodPost, path, nil, map[string]string{"reason": reason}, nil)
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
	var response struct {
		Recording terminalpkg.RecordingRef `json:"recording"`
	}
	path := terminalClientPath(workspace) + "/" + url.PathEscape(strings.TrimSpace(terminalID)) + "/recording"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, map[string]string{"action": action}, &response); err != nil {
		return terminalpkg.RecordingRef{}, err
	}
	return response.Recording, nil
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
