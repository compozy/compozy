package aghsdk

import (
	"context"
	"encoding/json"
	"strings"
)

// HostAPIMethod identifies one extension -> AGH Host API request.
type HostAPIMethod string

const (
	// HostAPIMethodSessionsList lists sessions visible to the extension.
	HostAPIMethodSessionsList HostAPIMethod = "sessions/list"
	// HostAPIMethodSessionsCreate creates a session.
	HostAPIMethodSessionsCreate HostAPIMethod = "sessions/create"
	// HostAPIMethodSessionsPrompt prompts a session.
	HostAPIMethodSessionsPrompt HostAPIMethod = "sessions/prompt"
	// HostAPIMethodSessionsStop stops a session.
	HostAPIMethodSessionsStop HostAPIMethod = "sessions/stop"
	// HostAPIMethodSessionsStatus returns session status.
	HostAPIMethodSessionsStatus HostAPIMethod = "sessions/status"
	// HostAPIMethodSessionsStatusGet returns authored-context session status.
	HostAPIMethodSessionsStatusGet HostAPIMethod = "sessions/status/get"
	// HostAPIMethodSessionsEvents returns session events.
	HostAPIMethodSessionsEvents HostAPIMethod = "sessions/events"
	// HostAPIMethodSessionsSoulRefresh refreshes a session's Soul snapshot through managed authoring.
	HostAPIMethodSessionsSoulRefresh HostAPIMethod = "sessions/soul/refresh"
	// HostAPIMethodSessionsHealthGet returns metadata-only session health.
	HostAPIMethodSessionsHealthGet HostAPIMethod = "sessions/health/get"
	// HostAPIMethodAgentsSoulGet returns a managed Soul read model.
	HostAPIMethodAgentsSoulGet HostAPIMethod = "agents/soul/get"
	// HostAPIMethodAgentsSoulValidate validates Soul content through the managed service.
	HostAPIMethodAgentsSoulValidate HostAPIMethod = "agents/soul/validate"
	// HostAPIMethodAgentsSoulPut writes SOUL.md through managed authoring.
	HostAPIMethodAgentsSoulPut HostAPIMethod = "agents/soul/put"
	// HostAPIMethodAgentsSoulDelete deletes SOUL.md through managed authoring.
	HostAPIMethodAgentsSoulDelete HostAPIMethod = "agents/soul/delete"
	// HostAPIMethodAgentsSoulHistory lists managed Soul authoring revisions.
	HostAPIMethodAgentsSoulHistory HostAPIMethod = "agents/soul/history"
	// HostAPIMethodAgentsSoulRollback rolls SOUL.md back through managed authoring.
	HostAPIMethodAgentsSoulRollback HostAPIMethod = "agents/soul/rollback"
	// HostAPIMethodAgentsHeartbeatGet returns a managed Heartbeat policy read model.
	HostAPIMethodAgentsHeartbeatGet HostAPIMethod = "agents/heartbeat/get"
	// HostAPIMethodAgentsHeartbeatValidate validates Heartbeat content through the managed service.
	HostAPIMethodAgentsHeartbeatValidate HostAPIMethod = "agents/heartbeat/validate"
	// HostAPIMethodAgentsHeartbeatPut writes HEARTBEAT.md through managed authoring.
	HostAPIMethodAgentsHeartbeatPut HostAPIMethod = "agents/heartbeat/put"
	// HostAPIMethodAgentsHeartbeatDelete deletes HEARTBEAT.md through managed authoring.
	HostAPIMethodAgentsHeartbeatDelete HostAPIMethod = "agents/heartbeat/delete"
	// HostAPIMethodAgentsHeartbeatHistory lists managed Heartbeat authoring revisions.
	HostAPIMethodAgentsHeartbeatHistory HostAPIMethod = "agents/heartbeat/history"
	// HostAPIMethodAgentsHeartbeatRollback rolls HEARTBEAT.md back through managed authoring.
	HostAPIMethodAgentsHeartbeatRollback HostAPIMethod = "agents/heartbeat/rollback"
	// HostAPIMethodAgentsHeartbeatStatus returns policy, session health, and wake audit status.
	HostAPIMethodAgentsHeartbeatStatus HostAPIMethod = "agents/heartbeat/status"
	// HostAPIMethodAgentsHeartbeatWake requests one managed advisory Heartbeat wake.
	HostAPIMethodAgentsHeartbeatWake HostAPIMethod = "agents/heartbeat/wake"
	// HostAPIMethodMemoryRecall recalls memory.
	HostAPIMethodMemoryRecall HostAPIMethod = "memory/recall"
	// HostAPIMethodMemoryStore stores memory.
	HostAPIMethodMemoryStore HostAPIMethod = "memory/store"
	// HostAPIMethodMemoryForget forgets memory.
	HostAPIMethodMemoryForget HostAPIMethod = "memory/forget"
	// HostAPIMethodObserveHealth returns daemon health.
	HostAPIMethodObserveHealth HostAPIMethod = "observe/health"
	// HostAPIMethodListLogs returns runtime logs.
	HostAPIMethodListLogs HostAPIMethod = "logs/list"
	// HostAPIMethodSkillsList lists skills.
	HostAPIMethodSkillsList HostAPIMethod = "skills/list"
	// HostAPIMethodNetworkStatus returns network runtime status.
	HostAPIMethodNetworkStatus HostAPIMethod = "network/status"
	// HostAPIMethodNetworkChannels lists network channels.
	HostAPIMethodNetworkChannels HostAPIMethod = "network/channels"
	// HostAPIMethodNetworkPeers lists visible network peers.
	HostAPIMethodNetworkPeers HostAPIMethod = "network/peers"
	// HostAPIMethodNetworkThreads lists public network threads.
	HostAPIMethodNetworkThreads HostAPIMethod = "network/threads"
	// HostAPIMethodNetworkThreadGet returns one public network thread.
	HostAPIMethodNetworkThreadGet HostAPIMethod = "network/thread/get"
	// HostAPIMethodNetworkThreadMessages lists messages inside one public network thread.
	HostAPIMethodNetworkThreadMessages HostAPIMethod = "network/thread/messages"
	// HostAPIMethodNetworkDirects lists network direct rooms.
	HostAPIMethodNetworkDirects HostAPIMethod = "network/directs"
	// HostAPIMethodNetworkDirectResolve resolves a deterministic two-party direct room.
	HostAPIMethodNetworkDirectResolve HostAPIMethod = "network/direct/resolve"
	// HostAPIMethodNetworkDirectMessages lists messages inside one direct room.
	HostAPIMethodNetworkDirectMessages HostAPIMethod = "network/direct/messages"
	// HostAPIMethodNetworkWorkGet returns one network work row.
	HostAPIMethodNetworkWorkGet HostAPIMethod = "network/work/get"
	// HostAPIMethodNetworkSend sends one network message.
	HostAPIMethodNetworkSend HostAPIMethod = "network/send"
	// HostAPIMethodResourcesList lists resources.
	HostAPIMethodResourcesList HostAPIMethod = "resources/list"
	// HostAPIMethodResourcesGet fetches one resource.
	HostAPIMethodResourcesGet HostAPIMethod = "resources/get"
	// HostAPIMethodResourcesSnapshot snapshots resources.
	HostAPIMethodResourcesSnapshot HostAPIMethod = "resources/snapshot"
	// HostAPIMethodClarifyAsk asks the operator one session-scoped question.
	HostAPIMethodClarifyAsk HostAPIMethod = "clarify/ask"
)

// HostAPI is a minimal typed client for extension Host API calls.
type HostAPI struct {
	transport Transport
	isReady   func() bool
}

// NewHostAPI creates a Host API client over a transport.
func NewHostAPI(transport Transport, isReady func() bool) *HostAPI {
	return newHostAPI(transport, isReady)
}

// Request calls a Host API method after readiness and sensitive-value checks.
func (h *HostAPI) Request(ctx context.Context, method HostAPIMethod, params any, result any) error {
	if h == nil || h.transport == nil {
		return NewNotInitializedError()
	}
	if h.isReady != nil && !h.isReady() {
		return NewNotInitializedError()
	}
	if strings.TrimSpace(string(method)) == "" {
		return NewInvalidParamsError("host api method is required", nil)
	}
	if path, ok := sensitiveJSONPath(params); ok {
		return NewInvalidParamsError("host api params contain sensitive value", map[string]any{
			"path": path,
		})
	}
	return h.transport.Call(ctx, string(method), params, result)
}

// RawRequest calls a Host API method and returns the raw JSON response.
func (h *HostAPI) RawRequest(ctx context.Context, method HostAPIMethod, params any) (json.RawMessage, error) {
	var result json.RawMessage
	if err := h.Request(ctx, method, params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

func newHostAPI(transport Transport, isReady func() bool) *HostAPI {
	return &HostAPI{transport: transport, isReady: isReady}
}

func sensitiveJSONPath(value any) (string, bool) {
	if value == nil {
		return "", false
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return "", false
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return "", false
	}
	return scanSensitive(decoded, "$")
}

func scanSensitive(value any, path string) (string, bool) {
	switch typed := value.(type) {
	case map[string]any:
		for key, entry := range typed {
			childPath := path + "." + key
			if sensitiveKey(key) {
				return childPath, true
			}
			if found, ok := scanSensitive(entry, childPath); ok {
				return found, true
			}
		}
	case []any:
		for _, entry := range typed {
			childPath := path + "[]"
			if found, ok := scanSensitive(entry, childPath); ok {
				return found, true
			}
		}
	case string:
		if strings.HasPrefix(typed, "agh_claim_") {
			return path, true
		}
	}
	return "", false
}

func sensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(key, "-", "_"))
	sensitive := []string{
		"access_token",
		"approval_token",
		"authorization",
		"bearer",
		"claim_token",
		"client_secret",
		"oauth_code",
		"password",
		"pkce",
		"refresh_token",
		"secret",
	}
	for _, item := range sensitive {
		if strings.Contains(normalized, item) {
			return true
		}
	}
	return false
}
