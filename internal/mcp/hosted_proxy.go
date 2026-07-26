package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"

	"github.com/compozy/agh/internal/tools"
	sdkmcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

const (
	hostedProxyAudioKey = "audio"
	hostedProxyImageKey = "image"
	hostedProxyTextKey  = "text"
)

// HostedProjectionHandler receives projection snapshots from the daemon stream.
type HostedProjectionHandler func(HostedProjectionResponse) error

// HostedProxyClient is the UDS client surface consumed by the hosted stdio proxy.
type HostedProxyClient interface {
	BindHostedMCP(ctx context.Context, req HostedBindRequest) (HostedBindResponse, error)
	HostedMCPProjection(ctx context.Context, bindID string) (HostedProjectionResponse, error)
	StreamHostedMCPProjection(
		ctx context.Context,
		bindID string,
		lastDigest string,
		handler HostedProjectionHandler,
	) error
	CallHostedMCP(ctx context.Context, req HostedCallRequest) (HostedCallResponse, error)
	ReleaseHostedMCP(ctx context.Context, req HostedReleaseRequest) error
}

// HostedProxyOptions configures one `agh tool mcp` stdio proxy process.
type HostedProxyOptions struct {
	SessionID string
	Nonce     string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Version   string
}

// RunHostedProxy binds to the daemon and serves hosted AGH tools over MCP stdio.
func RunHostedProxy(ctx context.Context, client HostedProxyClient, opts HostedProxyOptions) error {
	if ctx == nil {
		return errors.New("mcp: proxy context is required")
	}
	if client == nil {
		return errors.New("mcp: proxy client is required")
	}
	sessionID := strings.TrimSpace(opts.SessionID)
	if sessionID == "" {
		return ErrHostedSessionRequired
	}
	nonce := strings.TrimSpace(opts.Nonce)
	if nonce == "" {
		return ErrHostedNonceRequired
	}
	stdin := opts.Stdin
	if stdin == nil {
		return errors.New("mcp: proxy stdin is required")
	}
	stdout := opts.Stdout
	if stdout == nil {
		return errors.New("mcp: proxy stdout is required")
	}
	stderr := opts.Stderr
	if stderr == nil {
		stderr = io.Discard
	}
	version := strings.TrimSpace(opts.Version)
	if version == "" {
		version = "0.0.0"
	}

	bind, err := client.BindHostedMCP(ctx, HostedBindRequest{SessionID: sessionID, Nonce: nonce})
	if err != nil {
		return err
	}
	defer func() {
		releaseErr := client.ReleaseHostedMCP(context.Background(), HostedReleaseRequest{BindID: bind.BindID})
		if releaseErr != nil {
			log.New(stderr, "", 0).Printf("hosted MCP release failed: %v", releaseErr)
		}
	}()

	mcpServer := server.NewMCPServer(HostedServerName, version, server.WithToolCapabilities(true))
	applyHostedTools(mcpServer, client, bind.BindID, bind.Tools)

	proxyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var streamWG sync.WaitGroup
	streamWG.Go(func() {
		streamHostedProjection(proxyCtx, client, mcpServer, bind)
	})

	stdio := server.NewStdioServer(mcpServer)
	stdio.SetErrorLogger(log.New(stderr, "", 0))
	err = stdio.Listen(proxyCtx, stdin, stdout)
	cancel()
	streamWG.Wait()
	return err
}

func streamHostedProjection(
	ctx context.Context,
	client HostedProxyClient,
	mcpServer *server.MCPServer,
	initial HostedBindResponse,
) {
	lastDigest := strings.TrimSpace(initial.Digest)
	err := client.StreamHostedMCPProjection(
		ctx,
		initial.BindID,
		lastDigest,
		func(snapshot HostedProjectionResponse) error {
			if strings.TrimSpace(snapshot.Digest) == "" || snapshot.Digest == lastDigest {
				return nil
			}
			lastDigest = snapshot.Digest
			applyHostedTools(mcpServer, client, initial.BindID, snapshot.Tools)
			return nil
		},
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		return
	}
}

func applyHostedTools(
	mcpServer *server.MCPServer,
	client HostedProxyClient,
	bindID string,
	views []tools.ToolView,
) {
	if mcpServer == nil {
		return
	}
	toolsForServer := make([]server.ServerTool, 0, len(views))
	for i := range views {
		view := views[i]
		readOnly := view.Descriptor.ReadOnly
		destructive := view.Descriptor.Destructive
		openWorld := view.Descriptor.OpenWorld
		tool := sdkmcp.Tool{
			Name:            view.Descriptor.ID.String(),
			Title:           view.Descriptor.DisplayTitle,
			Description:     hostedToolDescription(view.Descriptor),
			RawInputSchema:  cloneRaw(view.Descriptor.InputSchema),
			RawOutputSchema: cloneRaw(view.Descriptor.OutputSchema),
			Annotations: sdkmcp.ToolAnnotation{
				Title:           view.Descriptor.DisplayTitle,
				ReadOnlyHint:    &readOnly,
				DestructiveHint: &destructive,
				OpenWorldHint:   &openWorld,
			},
			Meta: hostedToolMeta(view.Descriptor),
		}
		toolsForServer = append(toolsForServer, server.ServerTool{
			Tool: tool,
			Handler: func(ctx context.Context, req sdkmcp.CallToolRequest) (*sdkmcp.CallToolResult, error) {
				return callHostedTool(ctx, client, bindID, req)
			},
		})
	}
	mcpServer.SetTools(toolsForServer...)
}

func callHostedTool(
	ctx context.Context,
	client HostedProxyClient,
	bindID string,
	req sdkmcp.CallToolRequest,
) (*sdkmcp.CallToolResult, error) {
	rawInput, err := rawArguments(req.GetRawArguments())
	if err != nil {
		return sdkmcp.NewToolResultError(err.Error()), nil
	}
	response, err := client.CallHostedMCP(ctx, HostedCallRequest{
		BindID:     bindID,
		ToolName:   req.Params.Name,
		ToolCallID: hostedToolCallID(req),
		Input:      rawInput,
	})
	if err != nil {
		if partial, ok, partialErr := hostedToolPartialErrorResult(err); ok {
			if partialErr != nil {
				return sdkmcp.NewToolResultError(hostedToolErrorMessage(partialErr)), nil
			}
			return partial, nil
		}
		return sdkmcp.NewToolResultError(hostedToolErrorMessage(err)), nil
	}
	return hostedToolResult(response.Result)
}

func hostedToolDescription(descriptor tools.Descriptor) string {
	sections := make([]string, 0, 6)
	if title := strings.TrimSpace(descriptor.DisplayTitle); title != "" {
		if description := strings.TrimSpace(descriptor.Description); description != "" {
			sections = append(sections, title+"\n\n"+description)
		} else {
			sections = append(sections, title)
		}
	} else if description := strings.TrimSpace(descriptor.Description); description != "" {
		sections = append(sections, description)
	}

	if id := strings.TrimSpace(descriptor.ID.String()); id != "" {
		sections = append(sections, "AGH canonical tool ID: "+id)
	}
	if toolsets := hostedToolsetNames(descriptor.Toolsets); toolsets != "" {
		sections = append(sections, "AGH toolsets: "+toolsets)
	}
	if tags := hostedDescriptionValues(descriptor.Tags); tags != "" {
		sections = append(sections, "Tags: "+tags)
	}
	if hints := hostedDescriptionValues(descriptor.SearchHints); hints != "" {
		sections = append(sections, "Search hints: "+hints)
	}
	sections = append(sections, "Call the harness-returned tool reference.")
	return strings.Join(sections, "\n\n")
}

func hostedToolMeta(descriptor tools.Descriptor) *sdkmcp.Meta {
	fields := map[string]any{
		"anthropic/searchHint": hostedSearchHint(descriptor),
	}
	switch descriptor.ID {
	case tools.ToolIDToolList, tools.ToolIDToolSearch, tools.ToolIDToolInfo:
		fields["anthropic/alwaysLoad"] = true
	}
	return sdkmcp.NewMetaFromMap(fields)
}

func hostedSearchHint(descriptor tools.Descriptor) string {
	values := make([]string, 0, 6+len(descriptor.SearchHints)+len(descriptor.Tags)+len(descriptor.Toolsets))
	if id := strings.TrimSpace(descriptor.ID.String()); id != "" {
		values = append(values, id)
	}
	if title := strings.TrimSpace(descriptor.DisplayTitle); title != "" {
		values = append(values, title)
	}
	if description := strings.TrimSpace(descriptor.Description); description != "" {
		values = append(values, description)
	}
	values = append(values, cleanedValues(descriptor.SearchHints)...)
	values = append(values, cleanedValues(descriptor.Tags)...)
	for _, toolset := range descriptor.Toolsets {
		if value := strings.TrimSpace(toolset.String()); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, " | ")
}

func hostedToolsetNames(toolsets []tools.ToolsetID) string {
	values := make([]string, 0, len(toolsets))
	for _, toolset := range toolsets {
		if value := strings.TrimSpace(toolset.String()); value != "" {
			values = append(values, value)
		}
	}
	return strings.Join(values, ", ")
}

func hostedDescriptionValues(values []string) string {
	return strings.Join(cleanedValues(values), ", ")
}

func cleanedValues(values []string) []string {
	cleaned := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func rawArguments(args any) (json.RawMessage, error) {
	if args == nil {
		return json.RawMessage(`{}`), nil
	}
	if raw, ok := args.(json.RawMessage); ok {
		return cloneRaw(raw), nil
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("mcp: marshal hosted MCP arguments: %w", err)
	}
	if len(payload) == 0 || string(payload) == jsonNullLiteral {
		return json.RawMessage(`{}`), nil
	}
	return json.RawMessage(payload), nil
}

func hostedToolCallID(req sdkmcp.CallToolRequest) string {
	if req.Params.Meta == nil {
		return ""
	}
	if value, ok := req.Params.Meta.AdditionalFields["toolCallId"]; ok {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func hostedToolResult(result tools.ToolResult) (*sdkmcp.CallToolResult, error) {
	isError, err := hostedToolResultIsError(result)
	if err != nil {
		return nil, err
	}
	if len(result.Structured) > 0 {
		var structured any
		if err := json.Unmarshal(result.Structured, &structured); err == nil {
			converted := sdkmcp.NewToolResultStructured(structured, hostedResultFallback(result))
			return finishHostedToolResult(converted, result, isError)
		}
	}
	if len(result.Content) == 0 {
		converted := sdkmcp.NewToolResultText(hostedResultFallback(result))
		return finishHostedToolResult(converted, result, isError)
	}
	content := make([]sdkmcp.Content, 0, len(result.Content))
	for _, block := range result.Content {
		converted, err := hostedToolContent(block)
		if err != nil {
			return nil, err
		}
		if converted != nil {
			content = append(content, converted)
		}
	}
	if len(content) == 0 {
		converted := sdkmcp.NewToolResultText(hostedResultFallback(result))
		return finishHostedToolResult(converted, result, isError)
	}
	return finishHostedToolResult(&sdkmcp.CallToolResult{Content: content}, result, isError)
}

func finishHostedToolResult(
	converted *sdkmcp.CallToolResult,
	result tools.ToolResult,
	isError bool,
) (*sdkmcp.CallToolResult, error) {
	if converted == nil {
		return nil, errors.New("mcp: hosted tool result is required")
	}
	converted.IsError = isError
	if len(result.Artifacts) == 0 {
		return converted, nil
	}
	payload := struct {
		Artifacts []tools.ArtifactRef `json:"artifacts"`
		ReadTool  tools.ToolID        `json:"read_tool"`
	}{
		Artifacts: append([]tools.ArtifactRef(nil), result.Artifacts...),
		ReadTool:  tools.ToolIDToolArtifactRead,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("mcp: encode hosted tool artifact references: %w", err)
	}
	converted.Content = append(converted.Content, sdkmcp.NewTextContent(string(encoded)))
	converted.Meta = sdkmcp.NewMetaFromMap(map[string]any{
		"agh/artifacts": payload.Artifacts,
		"agh/readTool":  payload.ReadTool,
	})
	return converted, nil
}

type hostedPartialResultError interface {
	error
	PartialToolResult() *tools.ToolResult
}

func hostedToolPartialErrorResult(err error) (*sdkmcp.CallToolResult, bool, error) {
	if err == nil {
		return nil, false, nil
	}
	var partial *tools.ToolResult
	if toolErr, ok := errors.AsType[*tools.ToolError](err); ok {
		partial = toolErr.PartialResult
	} else if carrier, ok := errors.AsType[hostedPartialResultError](err); ok {
		partial = carrier.PartialToolResult()
	}
	if partial == nil {
		return nil, false, nil
	}
	converted, convertErr := hostedToolResult(*partial)
	if convertErr != nil {
		return nil, true, convertErr
	}
	converted.IsError = true
	converted.Content = append(converted.Content, sdkmcp.NewTextContent(hostedToolErrorMessage(err)))
	return converted, true, nil
}

func hostedToolResultIsError(result tools.ToolResult) (bool, error) {
	raw, ok := result.Metadata[toolResultIsErrorKey]
	if !ok || len(raw) == 0 {
		return false, nil
	}
	var isError bool
	if err := json.Unmarshal(raw, &isError); err != nil {
		return false, fmt.Errorf("mcp: decode hosted MCP error flag: %w", err)
	}
	return isError, nil
}

func hostedToolContent(block tools.ToolContent) (sdkmcp.Content, error) {
	switch strings.TrimSpace(block.Type) {
	case hostedProxyTextKey:
		return sdkmcp.NewTextContent(block.Text), nil
	case hostedProxyImageKey:
		data, err := hostedToolContentData(block, hostedProxyImageKey)
		if err != nil {
			return nil, err
		}
		return sdkmcp.NewImageContent(data, block.MIMEType), nil
	case hostedProxyAudioKey:
		data, err := hostedToolContentData(block, hostedProxyAudioKey)
		if err != nil {
			return nil, err
		}
		return sdkmcp.NewAudioContent(data, block.MIMEType), nil
	default:
		if len(block.Data) > 0 {
			return sdkmcp.NewTextContent(string(block.Data)), nil
		}
		if strings.TrimSpace(block.Text) != "" {
			return sdkmcp.NewTextContent(block.Text), nil
		}
	}
	return nil, nil
}

func hostedToolContentData(block tools.ToolContent, contentType string) (string, error) {
	var data string
	if err := json.Unmarshal(block.Data, &data); err != nil {
		return "", fmt.Errorf("mcp: decode hosted MCP %s content: %w", contentType, err)
	}
	return data, nil
}

func hostedResultFallback(result tools.ToolResult) string {
	if preview := strings.TrimSpace(result.Preview); preview != "" {
		return preview
	}
	if len(result.Structured) > 0 {
		return string(result.Structured)
	}
	if len(result.Content) > 0 {
		parts := make([]string, 0, len(result.Content))
		for _, block := range result.Content {
			if text := strings.TrimSpace(block.Text); text != "" {
				parts = append(parts, text)
			}
		}
		if len(parts) > 0 {
			return strings.Join(parts, "\n")
		}
	}
	return "{}"
}

func hostedToolErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	if toolErr, ok := errors.AsType[*tools.ToolError](err); ok {
		if len(toolErr.ReasonCodes) > 0 {
			return string(toolErr.ReasonCodes[0]) + ": " + toolErr.Error()
		}
		if toolErr.Code != "" {
			return string(toolErr.Code) + ": " + toolErr.Error()
		}
	}
	return err.Error()
}
