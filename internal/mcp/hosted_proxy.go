package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"strings"
	"sync"

	"github.com/compozy/compozy/internal/tools"
	sdkmcp "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	hostedProxyAudioKey = "audio"
	hostedProxyImageKey = "image"
	hostedProxyTextKey  = "text"
)

var hostedServerToolFingerprints sync.Map

type hostedToolRegistry interface {
	AddTool(tool *sdkmcp.Tool, handler sdkmcp.ToolHandler)
	RemoveTools(names ...string)
}

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

// HostedProxyOptions configures one `compozy tool mcp` stdio proxy process.
type HostedProxyOptions struct {
	SessionID string
	Nonce     string
	Stdin     io.Reader
	Stdout    io.Writer
	Stderr    io.Writer
	Version   string
}

// RunHostedProxy binds to the daemon and serves hosted Compozy tools over MCP stdio.
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
			slog.New(slog.NewTextHandler(stderr, nil)).Error("hosted MCP release failed", "error", releaseErr)
		}
	}()

	mcpServer := sdkmcp.NewServer(
		&sdkmcp.Implementation{Name: HostedServerName, Version: version},
		&sdkmcp.ServerOptions{
			Capabilities: &sdkmcp.ServerCapabilities{Tools: &sdkmcp.ToolCapabilities{}},
		},
	)
	defer hostedServerToolFingerprints.Delete(mcpServer)
	mcpServer.AddReceivingMiddleware(privateToolListCacheMiddleware())
	mcpServer.AddReceivingMiddleware(hostedProviderProtocolVersionMiddleware())
	applyHostedTools(mcpServer, client, bind.BindID, bind.Tools)

	proxyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var streamWG sync.WaitGroup
	streamWG.Go(func() {
		streamHostedProjection(proxyCtx, client, mcpServer, bind)
	})

	err = mcpServer.Run(
		proxyCtx,
		hostedProviderProtocolTransport{
			Transport: &sdkmcp.IOTransport{
				Reader: io.NopCloser(stdin),
				Writer: nopWriteCloser{Writer: stdout},
			},
		},
	)
	cancel()
	streamWG.Wait()
	return err
}

func streamHostedProjection(
	ctx context.Context,
	client HostedProxyClient,
	mcpServer *sdkmcp.Server,
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
	mcpServer *sdkmcp.Server,
	client HostedProxyClient,
	bindID string,
	views []tools.ToolView,
) {
	if mcpServer == nil {
		return
	}
	previous := hostedToolFingerprints(mcpServer)
	next := reconcileHostedTools(mcpServer, client, bindID, previous, views)
	setHostedToolFingerprints(mcpServer, next)
}

func reconcileHostedTools(
	registry hostedToolRegistry,
	client HostedProxyClient,
	bindID string,
	previous map[string]string,
	views []tools.ToolView,
) map[string]string {
	next := make(map[string]string, len(views))
	for i := range views {
		view := views[i]
		name := view.Descriptor.ID.String()
		tool := hostedMCPTool(view.Descriptor)
		fingerprint := hostedToolFingerprint(&tool)
		next[name] = fingerprint
		if fingerprint != "" && previous[name] == fingerprint {
			continue
		}
		registry.AddTool(
			&tool,
			func(
				ctx context.Context,
				req *sdkmcp.CallToolRequest,
			) (*sdkmcp.CallToolResult, error) {
				return callHostedTool(ctx, client, bindID, req)
			},
		)
	}
	stale := make([]string, 0, len(previous))
	for name := range previous {
		if _, retained := next[name]; !retained {
			stale = append(stale, name)
		}
	}
	if len(stale) > 0 {
		registry.RemoveTools(stale...)
	}
	return next
}

func hostedMCPTool(descriptor tools.Descriptor) sdkmcp.Tool {
	name := descriptor.ID.String()
	readOnly := descriptor.ReadOnly
	destructive := descriptor.Destructive
	openWorld := descriptor.OpenWorld
	return sdkmcp.Tool{
		Name:         name,
		Title:        descriptor.DisplayTitle,
		Description:  hostedToolDescription(descriptor),
		InputSchema:  cloneRaw(descriptor.InputSchema),
		OutputSchema: hostedOptionalSchema(descriptor.OutputSchema),
		Annotations: &sdkmcp.ToolAnnotations{
			Title:           descriptor.DisplayTitle,
			ReadOnlyHint:    readOnly,
			DestructiveHint: &destructive,
			OpenWorldHint:   &openWorld,
		},
		Meta: hostedToolMeta(descriptor),
	}
}

func hostedOptionalSchema(schema json.RawMessage) any {
	if len(schema) == 0 {
		return nil
	}
	return cloneRaw(schema)
}

func hostedToolFingerprint(tool *sdkmcp.Tool) string {
	payload, err := json.Marshal(tool)
	if err != nil {
		// Invalid raw schema JSON cannot be fingerprinted safely. An empty
		// fingerprint conservatively re-registers the tool on the next projection.
		return ""
	}
	digest := sha256.Sum256(payload)
	return fmt.Sprintf("%x", digest)
}

func callHostedTool(
	ctx context.Context,
	client HostedProxyClient,
	bindID string,
	req *sdkmcp.CallToolRequest,
) (*sdkmcp.CallToolResult, error) {
	rawInput, err := rawArguments(req.Params.Arguments)
	if err != nil {
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{&sdkmcp.TextContent{Text: err.Error()}},
			IsError: true,
		}, nil
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
				return &sdkmcp.CallToolResult{
					Content: []sdkmcp.Content{
						&sdkmcp.TextContent{Text: hostedToolErrorMessage(partialErr)},
					},
					IsError: true,
				}, nil
			}
			return partial, nil
		}
		return &sdkmcp.CallToolResult{
			Content: []sdkmcp.Content{
				&sdkmcp.TextContent{Text: hostedToolErrorMessage(err)},
			},
			IsError: true,
		}, nil
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
		sections = append(sections, "Compozy canonical tool ID: "+id)
	}
	if toolsets := hostedToolsetNames(descriptor.Toolsets); toolsets != "" {
		sections = append(sections, "Compozy toolsets: "+toolsets)
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

func hostedToolMeta(descriptor tools.Descriptor) sdkmcp.Meta {
	fields := map[string]any{
		"anthropic/searchHint": hostedSearchHint(descriptor),
	}
	switch descriptor.ID {
	case tools.ToolIDToolList, tools.ToolIDToolSearch, tools.ToolIDToolInfo:
		fields["anthropic/alwaysLoad"] = true
	}
	return fields
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

func hostedToolCallID(req *sdkmcp.CallToolRequest) string {
	if req == nil || req.Params == nil || req.Params.Meta == nil {
		return ""
	}
	if value, ok := req.Params.Meta["toolCallId"]; ok {
		if text, ok := value.(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func hostedToolFingerprints(server *sdkmcp.Server) map[string]string {
	if server == nil {
		return nil
	}
	value, ok := hostedServerToolFingerprints.Load(server)
	if !ok {
		return nil
	}
	fingerprints, ok := value.(map[string]string)
	if !ok {
		return nil
	}
	return maps.Clone(fingerprints)
}

func setHostedToolFingerprints(server *sdkmcp.Server, fingerprints map[string]string) {
	hostedServerToolFingerprints.Store(server, maps.Clone(fingerprints))
}
