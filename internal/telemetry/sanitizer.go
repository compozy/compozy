package telemetry

import (
	"regexp"
	"strings"

	"github.com/compozy/compozy/internal/redact"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/instrumentation"
	"go.opentelemetry.io/otel/sdk/resource"
)

// ModelClass classifies a gen_ai.request.model value for export. The zero value is ModelUnknown.
type ModelClass int

const (
	// ModelUnknown is a value no registry knows; it exports as "other".
	ModelUnknown ModelClass = iota
	// ModelCatalog is a trusted model-catalog entry (built-in or models.dev); it exports verbatim.
	ModelCatalog
	// ModelCustom is known only from user configuration; it exports as "custom".
	ModelCustom
)

// Registries are fixed when the exporters are built; values outside them are replaced or omitted
// (fail closed). Function registries may serve refreshed snapshots of their owning registry and must
// be safe for concurrent use: the span and metric exporters call them from their own goroutines.
type Registries struct {
	RouteTemplates []string // from the mounted gin engines (HTTP and UDS)
	ACPMethods     []string
	// HookEvents is the hook event taxonomy, injected because internal/hooks is instrumented with this
	// package and so cannot be imported by it.
	HookEvents []string
	ToolIDs    func(id string) bool   // native tool registry membership
	Providers  func(name string) bool // configured providers
	Agents     func(name string) bool // registered agent definitions
	// Models classifies a model value; nil classifies every value as ModelUnknown.
	Models func(provider, model string) ModelClass
	// IDs validates an identity attribute with its owner's canonical format; a key without a
	// validator is never exported.
	IDs map[attribute.Key]func(string) bool
	// Secrets reports whether s contains a registered secret; nil uses the internal/redact registry.
	Secrets func(s string) bool
}

// dbOperationPattern admits code-owned write operation and caller function names.
var dbOperationPattern = regexp.MustCompile(`^[A-Za-z0-9_./()*-]{1,160}$`)

// sanitizer rebuilds exported items from registered names and per-key value policies.
type sanitizer struct {
	reg        Registries
	routes     valueSet
	acpMethods valueSet
	hookEvents valueSet
	secrets    func(string) bool
	resource   *resource.Resource
	version    string
}

func newSanitizer(reg Registries, res *resource.Resource, version string) *sanitizer {
	secrets := reg.Secrets
	if secrets == nil {
		secrets = redact.ContainsRegisteredSecret
	}
	return &sanitizer{
		reg:        reg,
		routes:     newValueSet(reg.RouteTemplates...),
		acpMethods: newValueSet(reg.ACPMethods...),
		hookEvents: newValueSet(reg.HookEvents...),
		secrets:    secrets,
		resource:   res,
		version:    version,
	}
}

// spanName returns the exported name for name and the attribute keys that name may carry.
func (s *sanitizer) spanName(name string) (string, keySet) {
	if keys, ok := spanKeys[name]; ok {
		return name, keys
	}
	if method, ok := strings.CutPrefix(name, acpSpanPrefix); ok && s.acpMethods.has(method) {
		return name, acpSpanKeys
	}
	if method, ok := strings.CutPrefix(name, httpSpanPrefix); ok && httpMethods.has(method) {
		return name, httpSpanKeys
	}
	if method, route, ok := strings.Cut(name, " "); ok && httpMethods.has(method) && s.routes.has(route) {
		return name, httpSpanKeys
	}
	return UnknownName, unknownSpanKeys
}

// exportResource returns the code-built resource with every string value through the registered-secret
// check; it runs per export because secrets can be registered after the exporters are built.
func (s *sanitizer) exportResource() *resource.Resource {
	attrs := s.resource.Attributes()
	for i, kv := range attrs {
		if kv.Value.Type() == attribute.STRING {
			attrs[i] = s.final(kv.Key, kv.Value.AsString())
		}
	}
	return resource.NewSchemaless(attrs...)
}

// scope keeps only a registered scope name and stamps the daemon version.
func (s *sanitizer) scope(in instrumentation.Scope) instrumentation.Scope {
	name := UnknownName
	if scopeNames.has(in.Name) {
		name = in.Name
	}
	return instrumentation.Scope{Name: s.finalString(name), Version: s.finalString(s.version)}
}

// attributes rebuilds attrs: keys outside allowed are dropped and every value passes its policy.
func (s *sanitizer) attributes(attrs []attribute.KeyValue, allowed keySet) []attribute.KeyValue {
	if len(attrs) == 0 {
		return nil
	}
	provider := ""
	for _, kv := range attrs {
		if kv.Key == AttrGenAISystem && kv.Value.Type() == attribute.STRING {
			provider = kv.Value.AsString()
		}
	}
	clean := make([]attribute.KeyValue, 0, len(attrs))
	for _, kv := range attrs {
		if !allowed.has(kv.Key) {
			continue
		}
		if value, ok := s.attribute(kv, provider); ok {
			clean = append(clean, value)
		}
	}
	return clean
}

// attribute applies the per-key fallback policy: membership misses export "other", identities and
// enum or numeric values outside their set are omitted, and registered secrets export "invalid".
func (s *sanitizer) attribute(kv attribute.KeyValue, provider string) (attribute.KeyValue, bool) {
	switch kv.Key {
	case AttrProfileID, AttrWorkspaceID, AttrSessionID, AttrTurnID, AttrRunID, AttrLoopID, AttrAutomationID:
		return s.matching(kv, s.reg.IDs[kv.Key])
	case AttrAgent:
		return s.member(kv, s.reg.Agents), true
	case AttrGenAISystem:
		return s.member(kv, s.reg.Providers), true
	case AttrTool:
		return s.member(kv, s.reg.ToolIDs), true
	case AttrGenAIModel:
		return s.model(kv, provider), true
	case AttrDBCoalesced:
		return kv.Key.Bool(kv.Value.AsBool()), kv.Value.Type() == attribute.BOOL
	case AttrDBFlushes, AttrDBWriteMSTotal:
		value := kv.Value.AsInt64()
		return kv.Key.Int64(value), kv.Value.Type() == attribute.INT64 && value >= 0
	case AttrHTTPStatusCode:
		value := kv.Value.AsInt64()
		return kv.Key.Int64(value), kv.Value.Type() == attribute.INT64 && value >= 100 && value <= 599
	case AttrDBOperation:
		return s.matching(kv, dbOperationPattern.MatchString)
	default:
		return s.matching(kv, s.enum(kv.Key).has)
	}
}

func (s *sanitizer) enum(key attribute.Key) valueSet {
	switch key {
	case AttrOutcome:
		return outcomes
	case AttrTokenType:
		return tokenTypes
	case AttrCostStatus:
		return costStatuses
	case AttrToolRisk:
		return toolRisks
	case AttrErrorType:
		return errorClasses
	case AttrRPCSystem:
		return rpcSystems
	case AttrHTTPMethod:
		return httpMethods
	case AttrHookEvent:
		return s.hookEvents
	case AttrRPCMethod:
		return s.acpMethods
	case AttrHTTPRoute:
		return s.routes
	default:
		return nil
	}
}

// matching keeps a string value accepted by valid; anything else is omitted.
func (s *sanitizer) matching(kv attribute.KeyValue, valid func(string) bool) (attribute.KeyValue, bool) {
	if kv.Value.Type() != attribute.STRING || valid == nil || !valid(kv.Value.AsString()) {
		return attribute.KeyValue{}, false
	}
	return s.final(kv.Key, kv.Value.AsString()), true
}

// member keeps a registry member and replaces anything else with "other".
func (s *sanitizer) member(kv attribute.KeyValue, member func(string) bool) attribute.KeyValue {
	if kv.Value.Type() == attribute.STRING && member != nil && member(kv.Value.AsString()) {
		return s.final(kv.Key, kv.Value.AsString())
	}
	return s.final(kv.Key, ValueOther)
}

// model exports catalog models verbatim, configuration-only models as "custom", and others as "other".
func (s *sanitizer) model(kv attribute.KeyValue, provider string) attribute.KeyValue {
	// Metrics record ModelLabel's sentinels: they are classifications, not model values.
	if value := kv.Value.AsString(); kv.Value.Type() == attribute.STRING &&
		(value == ValueOther || value == ValueCustom) {
		return attribute.String(string(kv.Key), value)
	}
	class := ModelUnknown
	if kv.Value.Type() == attribute.STRING && s.reg.Models != nil {
		class = s.reg.Models(provider, kv.Value.AsString())
	}
	switch class {
	case ModelCatalog:
		return s.final(kv.Key, kv.Value.AsString())
	case ModelCustom:
		return s.final(kv.Key, ValueCustom)
	default:
		return s.final(kv.Key, ValueOther)
	}
}

// final is finalString for an attribute value.
func (s *sanitizer) final(key attribute.Key, value string) attribute.KeyValue {
	return key.String(s.finalString(value))
}

// finalString applies the registered-secret check, the last condition for every exported string.
func (s *sanitizer) finalString(value string) string {
	if s.secrets(value) {
		return ValueInvalid
	}
	return value
}
