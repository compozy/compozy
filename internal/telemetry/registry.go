// Package telemetry owns the daemon's OpenTelemetry export and its privacy boundary: every exported
// span and metric is rebuilt from the registered names, attribute keys, and value policies below.
package telemetry

import (
	"maps"

	"go.opentelemetry.io/otel/attribute"
)

// UnknownName replaces unregistered span names and instrumentation scopes.
const UnknownName = "compozy.unknown"

// Instrumentation scopes accepted by the sanitizing exporters. ScopeHTTP is the otelgin middleware
// scope shared by the HTTP and UDS servers.
const (
	ScopeHTTP       = "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	ScopeSession    = "github.com/compozy/compozy/internal/session"
	ScopeACP        = "github.com/compozy/compozy/internal/acp"
	ScopeMCP        = "github.com/compozy/compozy/internal/mcp"
	ScopeTools      = "github.com/compozy/compozy/internal/tools"
	ScopeStore      = "github.com/compozy/compozy/internal/store"
	ScopeObserve    = "github.com/compozy/compozy/internal/observe"
	ScopeHooks      = "github.com/compozy/compozy/internal/hooks"
	ScopeLoop       = "github.com/compozy/compozy/internal/loop"
	ScopeAutomation = "github.com/compozy/compozy/internal/automation"
	ScopeScheduler  = "github.com/compozy/compozy/internal/scheduler"
)

// Registered span names. ACP RPC spans are named "acp <method>" for a registered ACP method; HTTP
// spans are "<METHOD> <route template>" for a registered template, or "HTTP <METHOD>" when unmatched.
const (
	SpanSessionTurn       = "session.turn"
	SpanACPProcessSpawn   = "acp.process.spawn"
	SpanACPPermissionWait = "acp.permission.wait"
	SpanMCPHostedCall     = "mcp.hosted.call"
	SpanToolExecute       = "tool.execute"
	SpanDBWrite           = "db.write"
	SpanHookDispatch      = "hook.dispatch"
	SpanLoopRun           = "loop.run"
	SpanAutomationRun     = "automation.run"
	SpanSchedulerTick     = "scheduler.tick"
)

// Registered span event names; event attributes are never exported.
const (
	EventFirstToken = "first_token"
	EventRecovery   = "recovery"
)

// Registered attribute keys.
const (
	AttrProfileID      attribute.Key = "compozy.profile_id"
	AttrWorkspaceID    attribute.Key = "compozy.workspace_id"
	AttrSessionID      attribute.Key = "compozy.session_id"
	AttrTurnID         attribute.Key = "compozy.turn_id"
	AttrRunID          attribute.Key = "compozy.run_id"
	AttrLoopID         attribute.Key = "compozy.loop.id"
	AttrAutomationID   attribute.Key = "compozy.automation.id"
	AttrAgent          attribute.Key = "compozy.agent"
	AttrGenAISystem    attribute.Key = "gen_ai.system"
	AttrGenAIModel     attribute.Key = "gen_ai.request.model"
	AttrTool           attribute.Key = "compozy.tool"
	AttrToolRisk       attribute.Key = "compozy.tool.risk"
	AttrOutcome        attribute.Key = "compozy.outcome"
	AttrTokenType      attribute.Key = "compozy.token.type" // #nosec G101 -- attribute key, not a credential.
	AttrCostStatus     attribute.Key = "compozy.cost.status"
	AttrDBOperation    attribute.Key = "compozy.db.operation"
	AttrDBCoalesced    attribute.Key = "compozy.db.coalesced"
	AttrDBFlushes      attribute.Key = "compozy.db.flushes"
	AttrDBWriteMSTotal attribute.Key = "compozy.db.write_ms_total"
	AttrHookEvent      attribute.Key = "compozy.hook.event"
	AttrRPCSystem      attribute.Key = "rpc.system"
	AttrRPCMethod      attribute.Key = "rpc.method"
	AttrHTTPRoute      attribute.Key = "http.route"
	AttrHTTPMethod     attribute.Key = "http.request.method"
	AttrHTTPStatusCode attribute.Key = "http.response.status_code"
	AttrErrorType      attribute.Key = "error.type"
)

// Values substituted by the per-key fallback policy.
const (
	ValueOther   = "other"
	ValueCustom  = "custom"
	ValueInvalid = "invalid"
)

// Registered AttrOutcome values.
const (
	OutcomeOK          = "ok"
	OutcomeError       = "error"
	OutcomeCancelled   = "cancelled" //nolint:misspell // Exported catalog spelling (_dx.md).
	OutcomeApproved    = "approved"
	OutcomeDenied      = "denied"
	OutcomeUnbound     = "unbound"
	OutcomeAfterRun    = "after_run"
	OutcomeSetupFailed = "setup_failed"
)

// Registered AttrTokenType values.
const (
	TokenInput      = "input"
	TokenOutput     = "output"
	TokenCacheRead  = "cache_read"
	TokenCacheWrite = "cache_write"
)

// Registered AttrCostStatus values: provider-reported and catalog-estimated USD amounts.
const (
	CostActual    = "actual"
	CostEstimated = "estimated"
)

// RPCSystemJSONRPC is the only registered AttrRPCSystem value.
const RPCSystemJSONRPC = "jsonrpc"

// Registered metric instruments.
const (
	MetricSessionTurnDuration  = "compozy.session.turn.duration"
	MetricSessionTTFT          = "compozy.session.ttft"
	MetricACPRPCDuration       = "compozy.acp.rpc.duration"
	MetricToolCallDuration     = "compozy.tool.call.duration"
	MetricHTTPServerDuration   = "http.server.request.duration"
	MetricDBWriteDuration      = "compozy.db.write.duration"
	MetricDBWriteAdmissionWait = "compozy.db.write.admission_wait"
	MetricTokens               = "compozy.tokens" // #nosec G101 -- metric name, not a credential.
	MetricCost                 = "compozy.cost"
	MetricToolCalls            = "compozy.tool.calls"
	MetricPermissionRequests   = "compozy.permission.requests"
	MetricAgentErrors          = "compozy.agent.errors"
	MetricSessionActive        = "compozy.session.active"
	MetricAgentActive          = "compozy.agent.active"
	MetricUptime               = "compozy.uptime"
)

const (
	acpSpanPrefix  = "acp "
	httpSpanPrefix = "HTTP "
)

type keySet map[attribute.Key]struct{}

func newKeySet(keys ...attribute.Key) keySet {
	set := make(keySet, len(keys))
	for _, key := range keys {
		set[key] = struct{}{}
	}
	return set
}

func (s keySet) has(key attribute.Key) bool {
	_, ok := s[key]
	return ok
}

type valueSet map[string]struct{}

func newValueSet(values ...string) valueSet {
	set := make(valueSet, len(values))
	for _, value := range values {
		set[value] = struct{}{}
	}
	return set
}

func (s valueSet) has(value string) bool {
	_, ok := s[value]
	return ok
}

var (
	scopeNames = newValueSet(
		ScopeHTTP, ScopeSession, ScopeACP, ScopeMCP, ScopeTools, ScopeStore, ScopeObserve, ScopeHooks,
		ScopeLoop, ScopeAutomation, ScopeScheduler,
	)
	eventNames = newValueSet(EventFirstToken, EventRecovery)

	outcomes = newValueSet(
		OutcomeOK, OutcomeError, OutcomeCancelled, OutcomeApproved, OutcomeDenied, OutcomeUnbound,
		OutcomeAfterRun, OutcomeSetupFailed,
	)
	tokenTypes   = newValueSet(TokenInput, TokenOutput, TokenCacheRead, TokenCacheWrite)
	costStatuses = newValueSet(CostActual, CostEstimated)
	// toolRisks mirrors tools.RiskClass; internal/tools is instrumented with this package, so it cannot
	// be imported here.
	toolRisks    = newValueSet("read", "mutating", "open_world", "destructive")
	rpcSystems   = newValueSet(RPCSystemJSONRPC)
	errorClasses = newValueSet(
		ErrorCanceled, ErrorDeadlineExceeded, ErrorProvider, ErrorPermissionDenied, ErrorNotFound,
		ErrorInvalidInput, ErrorIO, ErrorInternal,
	)
	// httpMethods holds the RFC 9110 methods plus PATCH (RFC 5789).
	httpMethods = newValueSet(
		"GET", "HEAD", "POST", "PUT", "DELETE", "CONNECT", "OPTIONS", "TRACE", "PATCH",
	)

	// spanKeys is the per-span-name attribute allowlist; every registered span may carry error.type.
	spanKeys = map[string]keySet{
		SpanSessionTurn: newKeySet(
			AttrSessionID, AttrTurnID, AttrRunID, AttrWorkspaceID, AttrProfileID, AttrAgent, AttrGenAISystem,
			AttrGenAIModel, AttrOutcome, AttrDBFlushes, AttrDBWriteMSTotal, AttrErrorType,
		),
		SpanACPProcessSpawn:   newKeySet(AttrGenAISystem, AttrOutcome, AttrErrorType),
		SpanACPPermissionWait: newKeySet(AttrOutcome, AttrErrorType),
		SpanMCPHostedCall:     newKeySet(AttrTool, AttrRunID, AttrOutcome, AttrErrorType),
		SpanToolExecute:       newKeySet(AttrTool, AttrToolRisk, AttrOutcome, AttrErrorType),
		SpanDBWrite:           newKeySet(AttrDBOperation, AttrDBCoalesced, AttrOutcome, AttrErrorType),
		SpanHookDispatch:      newKeySet(AttrHookEvent, AttrOutcome, AttrErrorType),
		SpanLoopRun:           newKeySet(AttrLoopID, AttrErrorType),
		SpanAutomationRun:     newKeySet(AttrAutomationID, AttrErrorType),
		SpanSchedulerTick:     newKeySet(AttrErrorType),
	}
	acpSpanKeys  = newKeySet(AttrRPCSystem, AttrRPCMethod, AttrOutcome, AttrErrorType)
	httpSpanKeys = newKeySet(AttrHTTPRoute, AttrHTTPMethod, AttrHTTPStatusCode, AttrErrorType)
	// unknownSpanKeys lets a span with an unregistered name keep any registered span key, each value
	// still passing its policy.
	unknownSpanKeys = unionKeySets(spanKeys, acpSpanKeys, httpSpanKeys)
)

func unionKeySets(named map[string]keySet, extra ...keySet) keySet {
	union := newKeySet()
	for _, set := range named {
		maps.Copy(union, set)
	}
	for _, set := range extra {
		maps.Copy(union, set)
	}
	return union
}

// instrumentKind is the catalog metric type: an explicit-bucket histogram, a counter (monotonic sum),
// or a gauge (an OTel gauge, never an up-down counter).
type instrumentKind int

const (
	kindHistogram instrumentKind = iota + 1
	kindCounter
	kindGauge
)

type instrument struct {
	kind instrumentKind
	unit string
	keys keySet
}

// instruments is the metric catalog; metrics outside it, or of another type, are not exported.
var instruments = map[string]instrument{
	MetricSessionTurnDuration: {
		kind: kindHistogram, unit: "s", keys: newKeySet(AttrGenAISystem, AttrGenAIModel, AttrAgent, AttrOutcome),
	},
	MetricSessionTTFT: {kind: kindHistogram, unit: "s", keys: newKeySet(AttrGenAISystem, AttrGenAIModel)},
	MetricACPRPCDuration: {
		kind: kindHistogram, unit: "s", keys: newKeySet(AttrGenAISystem, AttrRPCMethod, AttrOutcome),
	},
	MetricToolCallDuration: {kind: kindHistogram, unit: "s", keys: newKeySet(AttrTool, AttrOutcome)},
	MetricHTTPServerDuration: {
		kind: kindHistogram, unit: "s", keys: newKeySet(AttrHTTPRoute, AttrHTTPMethod, AttrHTTPStatusCode),
	},
	MetricDBWriteDuration: {
		kind: kindHistogram, unit: "s", keys: newKeySet(AttrDBOperation, AttrDBCoalesced, AttrOutcome),
	},
	MetricDBWriteAdmissionWait: {kind: kindHistogram, unit: "s", keys: newKeySet(AttrDBOperation)},
	MetricTokens: {
		kind: kindCounter, unit: "{token}",
		keys: newKeySet(AttrGenAISystem, AttrGenAIModel, AttrAgent, AttrTokenType),
	},
	MetricCost: {
		kind: kindCounter, unit: "{USD}",
		keys: newKeySet(AttrGenAISystem, AttrGenAIModel, AttrAgent, AttrCostStatus),
	},
	MetricToolCalls:          {kind: kindCounter, unit: "{call}", keys: newKeySet(AttrTool, AttrOutcome)},
	MetricPermissionRequests: {kind: kindCounter, unit: "{request}", keys: newKeySet(AttrOutcome)},
	MetricAgentErrors:        {kind: kindCounter, unit: "{error}", keys: newKeySet(AttrGenAISystem, AttrErrorType)},
	MetricSessionActive:      {kind: kindGauge, unit: "{session}", keys: newKeySet()},
	MetricAgentActive:        {kind: kindGauge, unit: "{agent}", keys: newKeySet()},
	MetricUptime:             {kind: kindGauge, unit: "s", keys: newKeySet()},
}
