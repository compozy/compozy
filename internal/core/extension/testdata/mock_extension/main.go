package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/compozy/compozy/internal/core/subprocess"
)

type initializeRequest struct {
	ProtocolVersion     string   `json:"protocol_version"`
	GrantedCapabilities []string `json:"granted_capabilities"`
	Runtime             struct {
		RunID                 string `json:"run_id"`
		ParentRunID           string `json:"parent_run_id"`
		WorkspaceRoot         string `json:"workspace_root"`
		InvokingCommand       string `json:"invoking_command"`
		ShutdownTimeoutMS     int64  `json:"shutdown_timeout_ms"`
		DefaultHookTimeoutMS  int64  `json:"default_hook_timeout_ms"`
		HealthCheckIntervalMS int64  `json:"health_check_interval_ms"`
	} `json:"runtime"`
}

type initializeResponse struct {
	ProtocolVersion      string          `json:"protocol_version"`
	AcceptedCapabilities []string        `json:"accepted_capabilities,omitempty"`
	SupportedHookEvents  []string        `json:"supported_hook_events,omitempty"`
	Supports             initializeFlags `json:"supports"`
}

type initializeFlags struct {
	HealthCheck bool `json:"health_check"`
	OnEvent     bool `json:"on_event"`
}

type executeHookRequest struct {
	Hook struct {
		Name  string `json:"name"`
		Event string `json:"event"`
	} `json:"hook"`
	Payload json.RawMessage `json:"payload"`
}

type executeHookResponse struct {
	Patch json.RawMessage `json:"patch,omitempty"`
}

type onEventRequest struct {
	Event struct {
		Kind string `json:"kind"`
	} `json:"event"`
}

type healthResponse struct {
	Healthy bool   `json:"healthy"`
	Message string `json:"message,omitempty"`
}

type shutdownResponse struct {
	Acknowledged bool `json:"acknowledged"`
}

type record struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload,omitempty"`
}

type recorder struct {
	path string
}

func main() {
	transport := subprocess.NewTransport(os.Stdin, os.Stdout)
	if err := run(transport, recorder{path: os.Getenv("COMPOZY_MOCK_RECORD_PATH")}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(transport *subprocess.Transport, rec recorder) error {
	if transport == nil {
		return errors.New("missing transport")
	}

	mode := strings.TrimSpace(os.Getenv("COMPOZY_MOCK_MODE"))
	if mode == "ignore_shutdown" {
		trapSIGTERM()
	}

	initializeMessage, err := transport.ReadMessage()
	if err != nil {
		return err
	}
	if initializeMessage.Method != "initialize" || initializeMessage.ID == nil {
		return fmt.Errorf("expected initialize request, got %#v", initializeMessage)
	}

	var request initializeRequest
	if err := json.Unmarshal(initializeMessage.Params, &request); err != nil {
		return err
	}
	rec.write("initialize_env", map[string]any{
		"protocol_version":      os.Getenv("COMPOZY_PROTOCOL_VERSION"),
		"run_id":                os.Getenv("COMPOZY_RUN_ID"),
		"parent_run_id":         os.Getenv("COMPOZY_PARENT_RUN_ID"),
		"workspace_root":        os.Getenv("COMPOZY_WORKSPACE_ROOT"),
		"extension_name":        os.Getenv("COMPOZY_EXTENSION_NAME"),
		"extension_source":      os.Getenv("COMPOZY_EXTENSION_SOURCE"),
		"host_capability_token": os.Getenv("COMPOZY_HOST_CAPABILITY_TOKEN"),
	})
	rec.write("initialize_request", map[string]any{
		"protocol_version":      request.ProtocolVersion,
		"granted_capabilities":  request.GrantedCapabilities,
		"run_id":                request.Runtime.RunID,
		"parent_run_id":         request.Runtime.ParentRunID,
		"workspace_root":        request.Runtime.WorkspaceRoot,
		"invoking_command":      request.Runtime.InvokingCommand,
		"shutdown_timeout_ms":   request.Runtime.ShutdownTimeoutMS,
		"default_hook_timeout":  request.Runtime.DefaultHookTimeoutMS,
		"health_check_interval": request.Runtime.HealthCheckIntervalMS,
	})

	response := buildInitializeResponse(mode, request)
	if err := transport.WriteMessage(subprocess.Message{
		ID:     initializeMessage.ID,
		Result: mustMarshal(response),
	}); err != nil {
		return err
	}

	if mode == "host_tasks_list" {
		if err := callHostTasksList(transport, rec); err != nil {
			return err
		}
	}
	if mode == "exit_after_init" {
		return nil
	}

	healthChecks := 0
	for {
		message, err := transport.ReadMessage()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if message.Method == "" || message.ID == nil {
			continue
		}

		switch message.Method {
		case "execute_hook":
			var request executeHookRequest
			if err := json.Unmarshal(message.Params, &request); err != nil {
				return err
			}
			payload := map[string]any{}
			if len(request.Payload) > 0 {
				if err := json.Unmarshal(request.Payload, &payload); err != nil {
					payload = map[string]any{"raw": string(request.Payload)}
				}
			}
			rec.write("execute_hook", map[string]any{
				"name":    request.Hook.Name,
				"event":   request.Hook.Event,
				"payload": payload,
			})
			if err := transport.WriteMessage(subprocess.Message{
				ID:     message.ID,
				Result: mustMarshal(executeHookResponse{Patch: patchPayload(request)}),
			}); err != nil {
				return err
			}
		case "on_event":
			var request onEventRequest
			if err := json.Unmarshal(message.Params, &request); err != nil {
				return err
			}
			rec.write("on_event", map[string]any{"kind": request.Event.Kind})
			time.Sleep(durationFromEnv("COMPOZY_MOCK_ON_EVENT_DELAY_MS", 0))
			if err := transport.WriteMessage(subprocess.Message{
				ID:     message.ID,
				Result: mustMarshal(map[string]any{}),
			}); err != nil {
				return err
			}
		case "health_check":
			healthChecks++
			rec.write("health_check", map[string]any{"count": healthChecks})
			switch mode {
			case "health_false":
				if err := transport.WriteMessage(subprocess.Message{
					ID:     message.ID,
					Result: mustMarshal(healthResponse{Healthy: false, Message: "reported unhealthy"}),
				}); err != nil {
					return err
				}
			case "health_timeout":
				time.Sleep(durationFromEnv("COMPOZY_MOCK_HEALTH_DELAY_MS", 250*time.Millisecond))
				if err := transport.WriteMessage(subprocess.Message{
					ID:     message.ID,
					Result: mustMarshal(healthResponse{Healthy: true}),
				}); err != nil {
					return err
				}
			default:
				if err := transport.WriteMessage(subprocess.Message{
					ID:     message.ID,
					Result: mustMarshal(healthResponse{Healthy: true}),
				}); err != nil {
					return err
				}
			}
		case "shutdown":
			rec.write("shutdown", map[string]any{"mode": mode})
			if mode == "ignore_shutdown" {
				select {}
			}
			time.Sleep(durationFromEnv("COMPOZY_MOCK_SHUTDOWN_DELAY_MS", 0))
			if err := transport.WriteMessage(subprocess.Message{
				ID:     message.ID,
				Result: mustMarshal(shutdownResponse{Acknowledged: true}),
			}); err != nil {
				return err
			}
			return nil
		default:
			if err := transport.WriteMessage(subprocess.Message{
				ID:    message.ID,
				Error: subprocess.NewMethodNotFound(message.Method),
			}); err != nil {
				return err
			}
		}
	}
}

func buildInitializeResponse(mode string, request initializeRequest) initializeResponse {
	accepted := append([]string(nil), request.GrantedCapabilities...)
	switch mode {
	case "capability_mismatch":
		accepted = append(accepted, "memory.write")
	}

	response := initializeResponse{
		ProtocolVersion:      request.ProtocolVersion,
		AcceptedCapabilities: accepted,
		SupportedHookEvents:  splitCSV(os.Getenv("COMPOZY_MOCK_SUPPORTED_HOOKS")),
		Supports: initializeFlags{
			HealthCheck: mode == "health_false" || mode == "health_timeout" || os.Getenv("COMPOZY_MOCK_SUPPORTS_HEALTH") == "1",
			OnEvent:     contains(accepted, "events.read"),
		},
	}

	switch mode {
	case "unsupported_protocol":
		response.ProtocolVersion = "2"
	case "events_without_on_event":
		if !contains(accepted, "events.read") {
			accepted = append(accepted, "events.read")
		}
		response.AcceptedCapabilities = accepted
		response.Supports.OnEvent = false
	}

	return response
}

func callHostTasksList(transport *subprocess.Transport, rec recorder) error {
	workflow := strings.TrimSpace(os.Getenv("COMPOZY_MOCK_WORKFLOW"))
	requestID := json.RawMessage(`"mock-host-tasks-list"`)
	if err := transport.WriteMessage(subprocess.Message{
		ID:     &requestID,
		Method: "host.tasks.list",
		Params: mustMarshal(map[string]any{"workflow": workflow}),
	}); err != nil {
		return err
	}

	response, err := transport.ReadMessage()
	if err != nil {
		return err
	}
	payload := map[string]any{}
	if response.Error != nil {
		payload["error"] = response.Error.Error()
	} else {
		var tasks []map[string]any
		if err := json.Unmarshal(response.Result, &tasks); err != nil {
			return err
		}
		payload["count"] = len(tasks)
		if len(tasks) > 0 {
			payload["first_path"] = tasks[0]["path"]
		}
	}
	rec.write("host_tasks_list", payload)
	return nil
}

func (r recorder) write(kind string, payload map[string]any) {
	if strings.TrimSpace(r.path) == "" {
		return
	}

	line, err := json.Marshal(record{Type: kind, Payload: payload})
	if err != nil {
		return
	}

	file, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = file.Write(append(line, '\n'))
}

func trapSIGTERM() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM)
	go func() {
		for range ch {
		}
	}()
}

func splitCSV(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			items = append(items, part)
		}
	}
	return items
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func patchPayload(request executeHookRequest) json.RawMessage {
	if patch := appendPatchPayload(request); len(patch) > 0 {
		return patch
	}

	raw := strings.TrimSpace(os.Getenv("COMPOZY_MOCK_PATCH_JSON"))
	if raw == "" {
		return json.RawMessage(`{}`)
	}
	return json.RawMessage(raw)
}

func appendPatchPayload(request executeHookRequest) json.RawMessage {
	raw := strings.TrimSpace(os.Getenv("COMPOZY_MOCK_APPEND_SUFFIXES_JSON"))
	if raw == "" {
		return nil
	}

	var suffixes map[string]string
	if err := json.Unmarshal([]byte(raw), &suffixes); err != nil {
		return nil
	}
	suffix := suffixes[strings.TrimSpace(request.Hook.Event)]
	if suffix == "" {
		return nil
	}

	var payload map[string]any
	if err := json.Unmarshal(request.Payload, &payload); err != nil {
		return nil
	}

	switch {
	case updatePromptText(payload, suffix):
		return mustMarshal(map[string]any{"prompt_text": payload["prompt_text"]})
	case updateSystemAddendum(payload, suffix):
		return mustMarshal(map[string]any{"system_addendum": payload["system_addendum"]})
	case updateEntries(payload, suffix):
		return mustMarshal(map[string]any{"entries": payload["entries"]})
	case updateSessionRequestPrompt(payload, suffix):
		return mustMarshal(map[string]any{"session_request": payload["session_request"]})
	default:
		return nil
	}
}

func updatePromptText(payload map[string]any, suffix string) bool {
	current, ok := payload["prompt_text"].(string)
	if !ok {
		return false
	}
	payload["prompt_text"] = current + suffix
	return true
}

func updateSystemAddendum(payload map[string]any, suffix string) bool {
	current, ok := payload["system_addendum"].(string)
	if !ok {
		return false
	}
	payload["system_addendum"] = current + suffix
	return true
}

func updateEntries(payload map[string]any, suffix string) bool {
	rawEntries, ok := payload["entries"].([]any)
	if !ok {
		return false
	}

	updated := false
	for _, rawEntry := range rawEntries {
		entry, ok := rawEntry.(map[string]any)
		if !ok {
			continue
		}
		current, ok := entry["Content"].(string)
		if !ok {
			continue
		}
		entry["Content"] = current + suffix
		updated = true
	}
	if updated {
		payload["entries"] = rawEntries
	}
	return updated
}

func updateSessionRequestPrompt(payload map[string]any, suffix string) bool {
	rawSessionRequest, ok := payload["session_request"].(map[string]any)
	if !ok {
		return false
	}
	currentPrompt, ok := rawSessionRequest["prompt"].(string)
	if !ok {
		return false
	}
	rawSessionRequest["prompt"] = currentPrompt + suffix
	payload["session_request"] = rawSessionRequest
	return true
}

func durationFromEnv(name string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(name))
	if value == "" {
		return fallback
	}
	ms, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return time.Duration(ms) * time.Millisecond
}

func mustMarshal(value any) json.RawMessage {
	raw, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return raw
}

var _ = context.Background
