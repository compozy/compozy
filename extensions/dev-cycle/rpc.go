package devcycle

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"

	watchpkg "github.com/compozy/agh/internal/loop/watch"
	"github.com/compozy/agh/internal/subprocess"
	toolspkg "github.com/compozy/agh/internal/tools"
)

const (
	jsonRPCVersion        = "2.0"
	devCycleSDKName       = "agh-dev-cycle"
	rpcMethodInitialize   = "initialize"
	rpcMethodHealthCheck  = "health_check"
	rpcMethodProvideTools = "provide_tools"
	rpcMethodShutdown     = "shutdown"
	rpcMethodToolsCall    = "tools/call"
	rpcMethodWatchPoll    = "watch/poll"
	watchKindCodeRabbitPR = "coderabbit_pr_review"
)

// RunProvider serves the dev-cycle extension subprocess protocol over stdio.
func RunProvider(ctx context.Context, stdin io.Reader, stdout io.Writer) error {
	if ctx == nil {
		ctx = context.Background()
	}
	server, err := newRPCServer(stdout)
	if err != nil {
		return err
	}
	return server.run(ctx, stdin)
}

type rpcServer struct {
	writer  *bufio.Writer
	runtime *runtimeProvider
	mu      sync.Mutex
}

type rpcRequest struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func newRPCServer(stdout io.Writer) (*rpcServer, error) {
	if stdout == nil {
		return nil, errors.New("dev-cycle: stdout is required")
	}
	runtime, err := newRuntimeProvider()
	if err != nil {
		return nil, err
	}
	return &rpcServer{
		writer:  bufio.NewWriter(stdout),
		runtime: runtime,
	}, nil
}

func (s *rpcServer) run(ctx context.Context, stdin io.Reader) error {
	if stdin == nil {
		return errors.New("dev-cycle: stdin is required")
	}
	scanner := bufio.NewScanner(stdin)
	scanner.Buffer(make([]byte, 0, 1024), 10<<20)
	for scanner.Scan() {
		if err := ctx.Err(); err != nil {
			return err
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var request rpcRequest
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			continue
		}
		shouldStop, err := s.handle(ctx, request)
		if err != nil {
			return err
		}
		if shouldStop {
			return nil
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("dev-cycle: read rpc stream: %w", err)
	}
	return nil
}

func (s *rpcServer) handle(ctx context.Context, request rpcRequest) (bool, error) {
	switch request.Method {
	case rpcMethodInitialize:
		var params subprocess.InitializeRequest
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return false, s.sendError(request.ID, -32602, fmt.Sprintf("decode initialize params: %v", err))
		}
		return false, s.sendResult(request.ID, initializeResponse(params))
	case rpcMethodHealthCheck:
		return false, s.sendResult(request.ID, subprocess.HealthCheckResponse{Healthy: true})
	case rpcMethodProvideTools:
		tools, err := s.runtime.ProvideTools()
		if err != nil {
			return false, s.sendError(request.ID, -32000, err.Error())
		}
		return false, s.sendResult(request.ID, toolspkg.ExtensionProvideToolsResponse{Tools: tools})
	case rpcMethodToolsCall:
		var params toolspkg.ExtensionToolCallRequest
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return false, s.sendError(request.ID, -32602, fmt.Sprintf("decode tools/call params: %v", err))
		}
		result, err := s.runtime.CallTool(ctx, params)
		if err != nil {
			return false, s.sendToolError(request.ID, -32010, err)
		}
		return false, s.sendResult(request.ID, toolspkg.ExtensionToolCallResponse{Result: result})
	case rpcMethodWatchPoll:
		var params watchpkg.PollRequest
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return false, s.sendError(request.ID, -32602, fmt.Sprintf("decode watch/poll params: %v", err))
		}
		response, err := s.runtime.Poll(ctx, params)
		if err != nil {
			return false, s.sendError(request.ID, -32020, err.Error())
		}
		return false, s.sendResult(request.ID, response)
	case rpcMethodShutdown:
		if err := s.sendResult(request.ID, subprocess.ShutdownResponse{Acknowledged: true}); err != nil {
			return false, err
		}
		return true, nil
	default:
		return false, s.sendError(request.ID, -32601, "method not found")
	}
}

func initializeResponse(req subprocess.InitializeRequest) subprocess.InitializeResponse {
	methods := []string{rpcMethodHealthCheck, rpcMethodShutdown}
	methods = append(methods, req.Methods.ExtensionServices...)
	slices.Sort(methods)
	methods = slices.Compact(methods)
	return subprocess.InitializeResponse{
		ProtocolVersion: req.ProtocolVersion,
		ExtensionInfo: subprocess.InitializeExtensionInfo{
			Name:    req.Extension.Name,
			Version: req.Extension.Version,
			SDKName: devCycleSDKName,
		},
		AcceptedCapabilities: subprocess.AcceptedCapabilities{
			Provides: slices.Clone(req.Capabilities.Provides),
			Actions:  slices.Clone(req.Capabilities.GrantedActions),
			Security: slices.Clone(req.Capabilities.GrantedSecurity),
		},
		ImplementedMethods: methods,
		WatchSourceKinds:   []string{watchKindCodeRabbitPR},
		Supports: subprocess.InitializeSupports{
			HealthCheck: true,
		},
	}
}

func (s *rpcServer) sendResult(id json.RawMessage, result any) error {
	return s.write(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      cloneRawMessage(id),
		Result:  result,
	})
}

func (s *rpcServer) sendError(id json.RawMessage, code int, message string) error {
	return s.sendErrorData(id, code, message, nil)
}

func (s *rpcServer) sendToolError(id json.RawMessage, code int, err error) error {
	var toolErr *toolspkg.ToolError
	if !errors.As(err, &toolErr) {
		return s.sendError(id, code, err.Error())
	}
	data, marshalErr := json.Marshal(toolErr)
	if marshalErr != nil {
		return fmt.Errorf("dev-cycle: encode tool error: %w", marshalErr)
	}
	return s.sendErrorData(id, code, toolErr.Error(), data)
}

func (s *rpcServer) sendErrorData(
	id json.RawMessage,
	code int,
	message string,
	data json.RawMessage,
) error {
	return s.write(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      cloneRawMessage(id),
		Error: &rpcError{
			Code:    code,
			Message: strings.TrimSpace(message),
			Data:    cloneRawMessage(data),
		},
	})
}

func (s *rpcServer) write(response rpcResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := json.NewEncoder(s.writer).Encode(response); err != nil {
		return fmt.Errorf("dev-cycle: encode rpc response: %w", err)
	}
	if err := s.writer.Flush(); err != nil {
		return fmt.Errorf("dev-cycle: flush rpc response: %w", err)
	}
	return nil
}

func cloneRawMessage(src json.RawMessage) json.RawMessage {
	if len(src) == 0 {
		return nil
	}
	out := make(json.RawMessage, len(src))
	copy(out, src)
	return out
}

func runtimeToolID(handler string) (toolspkg.ToolID, error) {
	return toolspkg.CanonicalToolID("ext", Name, handler)
}
