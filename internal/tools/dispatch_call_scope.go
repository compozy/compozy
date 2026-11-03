package tools

import (
	"context"
	"strings"
	"sync"

	redactpkg "github.com/compozy/compozy/internal/redact"
)

// callScopeAccess records, for one dispatched call, what the workspace-access boundary
// authorized beyond the caller's own workspace and profile.
type callScopeAccess struct {
	mu       sync.Mutex
	outside  bool
	sessions map[string]struct{}
}

type callScopeAccessKey struct{}

// withCallScopeAccess attaches a fresh per-call record. Only dispatch creates one, so a
// handler can trust what it reads back.
func withCallScopeAccess(ctx context.Context) (context.Context, *callScopeAccess) {
	access := &callScopeAccess{}
	return context.WithValue(ctx, callScopeAccessKey{}, access), access
}

func callScopeAccessFrom(ctx context.Context) *callScopeAccess {
	if ctx == nil {
		return nil
	}
	access, ok := ctx.Value(callScopeAccessKey{}).(*callScopeAccess)
	if !ok {
		return nil
	}
	return access
}

// MarkOutsideCallerScope records that the boundary admitted this call to data owned by
// another workspace, profile, or All projects. Dispatch then redacts the call's result.
func MarkOutsideCallerScope(ctx context.Context) {
	if access := callScopeAccessFrom(ctx); access != nil {
		access.mu.Lock()
		access.outside = true
		access.mu.Unlock()
	}
}

// MarkSessionTargetAuthorized records that the boundary authorized the named target session
// against its own workspace and profile. It also marks the call as outside the caller scope.
func MarkSessionTargetAuthorized(ctx context.Context, sessionID string) {
	access := callScopeAccessFrom(ctx)
	sessionID = strings.TrimSpace(sessionID)
	if access == nil || sessionID == "" {
		return
	}
	access.mu.Lock()
	defer access.mu.Unlock()
	access.outside = true
	if access.sessions == nil {
		access.sessions = make(map[string]struct{})
	}
	access.sessions[sessionID] = struct{}{}
}

// SessionTargetAuthorized reports whether the dispatch boundary authorized sessionID for this call.
func SessionTargetAuthorized(ctx context.Context, sessionID string) bool {
	access := callScopeAccessFrom(ctx)
	if access == nil {
		return false
	}
	access.mu.Lock()
	defer access.mu.Unlock()
	_, ok := access.sessions[strings.TrimSpace(sessionID)]
	return ok
}

func (a *callScopeAccess) outsideCallerScope() bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.outside
}

// redactOutsideScopeResult removes secret material from a result the caller read outside its
// own workspace or profile. Tool inputs and outputs, messages, and previews are all strings in
// the envelope, so every string is redacted except structural identifiers.
func redactOutsideScopeResult(result ToolResult) ToolResult {
	redacted := cloneToolResult(result)
	redacted.Preview = redactpkg.CrossScopeString(redacted.Preview)
	redacted.Structured = crossScopeRawJSON(redacted.Structured)
	for i := range redacted.Content {
		redacted.Content[i].Text = redactpkg.CrossScopeString(redacted.Content[i].Text)
		redacted.Content[i].Data = crossScopeRawJSON(redacted.Content[i].Data)
		crossScopeRawMap(redacted.Content[i].Metadata)
	}
	crossScopeRawMap(redacted.Metadata)
	return redacted
}

func crossScopeRawJSON(raw []byte) []byte {
	if len(raw) == 0 {
		return raw
	}
	return redactpkg.CrossScopeJSON(raw)
}

func crossScopeRawMap[M ~map[string]V, V ~[]byte](values M) {
	for key, value := range values {
		values[key] = V(crossScopeRawJSON(value))
	}
}
