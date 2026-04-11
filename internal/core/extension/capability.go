package extensions

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/compozy/compozy/internal/core/subprocess"
)

const (
	capabilityDeniedCode    = -32001
	capabilityDeniedMessage = "Capability denied"
)

var hostMethodCapabilities = map[string]Capability{
	"host.events.subscribe": CapabilityEventsRead,
	"host.events.publish":   CapabilityEventsPublish,
	"host.tasks.list":       CapabilityTasksRead,
	"host.tasks.get":        CapabilityTasksRead,
	"host.tasks.create":     CapabilityTasksCreate,
	"host.runs.start":       CapabilityRunsStart,
	"host.artifacts.read":   CapabilityArtifactsRead,
	"host.artifacts.write":  CapabilityArtifactsWrite,
	"host.prompts.render":   "",
	"host.memory.read":      CapabilityMemoryRead,
	"host.memory.write":     CapabilityMemoryWrite,
}

// CapabilityChecker authorizes hook dispatches and Host API calls against the
// capabilities accepted during the initialize handshake.
type CapabilityChecker struct {
	granted     capabilitySet
	grantedList []Capability
}

// UnknownCapabilityTargetError reports attempts to authorize an unknown method
// or hook event.
type UnknownCapabilityTargetError struct {
	Kind string
	Name string
}

// Error returns a stable human-readable error string.
func (e *UnknownCapabilityTargetError) Error() string {
	if e == nil {
		return "<nil>"
	}
	if strings.TrimSpace(e.Kind) == "" {
		return fmt.Sprintf("unknown capability target %q", e.Name)
	}
	return fmt.Sprintf("unknown capability %s %q", e.Kind, e.Name)
}

// CapabilityDeniedError reports a structured capability denial.
type CapabilityDeniedError struct {
	Method  string
	Missing []Capability
	Granted []Capability
}

type capabilityDeniedPayload struct {
	Code    int                       `json:"code"`
	Message string                    `json:"message"`
	Data    capabilityDeniedErrorData `json:"data"`
}

type capabilityDeniedErrorData struct {
	Method   string       `json:"method"`
	Required []Capability `json:"required"`
	Granted  []Capability `json:"granted"`
}

// Error returns a stable human-readable error string.
func (e *CapabilityDeniedError) Error() string {
	if e == nil {
		return "<nil>"
	}

	missing := make([]string, 0, len(e.Missing))
	for _, capability := range e.Missing {
		missing = append(missing, string(capability))
	}

	granted := make([]string, 0, len(e.Granted))
	for _, capability := range e.Granted {
		granted = append(granted, string(capability))
	}

	return fmt.Sprintf(
		"capability denied for %q: missing=%v granted=%v",
		e.Method,
		missing,
		granted,
	)
}

// MarshalJSON serializes the error into the JSON-RPC error payload described by
// the extension protocol.
func (e *CapabilityDeniedError) MarshalJSON() ([]byte, error) {
	if e == nil {
		return []byte("null"), nil
	}

	return json.Marshal(capabilityDeniedPayload{
		Code:    capabilityDeniedCode,
		Message: capabilityDeniedMessage,
		Data:    e.data(),
	})
}

// RequestError converts the denial to the shared JSON-RPC error type used by
// the subprocess transport.
func (e *CapabilityDeniedError) RequestError() *subprocess.RequestError {
	if e == nil {
		return nil
	}

	return &subprocess.RequestError{
		Code:    capabilityDeniedCode,
		Message: capabilityDeniedMessage,
		Data:    e.data(),
	}
}

func (e *CapabilityDeniedError) data() capabilityDeniedErrorData {
	return capabilityDeniedErrorData{
		Method:   strings.TrimSpace(e.Method),
		Required: cloneCapabilities(e.Missing),
		Granted:  cloneCapabilities(e.Granted),
	}
}

// NewCapabilityChecker constructs a pure capability checker for one extension
// session.
func NewCapabilityChecker(accepted []Capability) CapabilityChecker {
	granted := make(capabilitySet, len(accepted))
	grantedList := make([]Capability, 0, len(accepted))
	for _, capability := range accepted {
		normalized := normalizeCapability(capability)
		if normalized == "" {
			continue
		}
		if _, exists := granted[normalized]; exists {
			continue
		}
		granted[normalized] = struct{}{}
		grantedList = append(grantedList, normalized)
	}
	sortCapabilities(grantedList)

	return CapabilityChecker{
		granted:     granted,
		grantedList: grantedList,
	}
}

// Granted returns the accepted capability list in a stable order.
func (c CapabilityChecker) Granted() []Capability {
	return cloneCapabilities(c.grantedList)
}

// Has reports whether the accepted capability set contains one capability.
func (c CapabilityChecker) Has(capability Capability) bool {
	if c.granted == nil {
		return false
	}
	_, ok := c.granted[normalizeCapability(capability)]
	return ok
}

// Check validates an arbitrary capability requirement list.
func (c CapabilityChecker) Check(target string, required ...Capability) error {
	missing := make([]Capability, 0, len(required))
	seen := make(capabilitySet, len(required))
	for _, capability := range required {
		normalized := normalizeCapability(capability)
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		if c.Has(normalized) {
			continue
		}
		missing = append(missing, normalized)
	}
	if len(missing) == 0 {
		return nil
	}
	sortCapabilities(missing)

	return &CapabilityDeniedError{
		Method:  strings.TrimSpace(target),
		Missing: missing,
		Granted: c.Granted(),
	}
}

// CheckHostMethod validates one Host API call against the accepted capabilities.
func (c CapabilityChecker) CheckHostMethod(method string) error {
	required, ok := capabilityForHostMethod(method)
	if !ok {
		return &UnknownCapabilityTargetError{Kind: "method", Name: strings.TrimSpace(method)}
	}
	return c.Check(method, required)
}

// CheckHook validates one hook event against the accepted capabilities.
func (c CapabilityChecker) CheckHook(hook HookName) error {
	normalized := HookName(strings.TrimSpace(string(hook)))
	if !supportedHookNames.contains(normalized) {
		return &UnknownCapabilityTargetError{Kind: "hook", Name: string(normalized)}
	}
	return c.Check(string(normalized), capabilityForHook(normalized))
}

// WarnCapabilityDenied emits the standardized slog warning that downstream
// runtime components use when a capability denial occurs.
func WarnCapabilityDenied(component, extension string, err error) {
	var denied *CapabilityDeniedError
	if !errors.As(err, &denied) || denied == nil {
		return
	}

	component = strings.TrimSpace(component)
	if component == "" {
		component = "extension.security"
	}

	slog.Warn(
		"extension capability denied",
		"component", component,
		"action", "capability_denied",
		"extension", strings.TrimSpace(extension),
		"method", denied.Method,
		"missing", denied.Missing,
		"granted", denied.Granted,
	)
}

func capabilityForHostMethod(method string) (Capability, bool) {
	required, ok := hostMethodCapabilities[strings.TrimSpace(method)]
	return required, ok
}

func capabilityForHook(hook HookName) Capability {
	switch {
	case strings.HasPrefix(string(hook), "plan."):
		return CapabilityPlanMutate
	case strings.HasPrefix(string(hook), "prompt."):
		return CapabilityPromptMutate
	case strings.HasPrefix(string(hook), "agent."):
		return CapabilityAgentMutate
	case strings.HasPrefix(string(hook), "job."):
		return CapabilityJobMutate
	case strings.HasPrefix(string(hook), "run."):
		return CapabilityRunMutate
	case strings.HasPrefix(string(hook), "review."):
		return CapabilityReviewMutate
	case strings.HasPrefix(string(hook), "artifact."):
		return CapabilityArtifactsWrite
	default:
		return ""
	}
}

func cloneCapabilities(values []Capability) []Capability {
	if len(values) == 0 {
		return nil
	}
	return slices.Clone(values)
}

func normalizeCapability(capability Capability) Capability {
	return Capability(strings.TrimSpace(string(capability)))
}

func sortCapabilities(values []Capability) {
	slices.SortFunc(values, func(left, right Capability) int {
		return strings.Compare(string(left), string(right))
	})
}
