package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const executeHookMethod = "execute_hook"

// HookDispatcher routes hook invocations across the runtime extension chain for
// one run.
type HookDispatcher struct {
	audit      AuditHandler
	chains     map[HookName][]hookChainEntry
	invocation atomic.Uint64
	pending    sync.WaitGroup
}

type executeHookRequest struct {
	InvocationID string                 `json:"invocation_id"`
	Hook         executeHookRequestHook `json:"hook"`
	Payload      any                    `json:"payload"`
}

type executeHookRequestHook struct {
	Name      string   `json:"name"`
	Event     HookName `json:"event"`
	Mutable   bool     `json:"mutable"`
	Required  bool     `json:"required"`
	Priority  int      `json:"priority"`
	TimeoutMS int64    `json:"timeout_ms"`
}

type executeHookResponse struct {
	Patch json.RawMessage `json:"patch,omitempty"`
}

// NewHookDispatcher builds the frozen per-event hook chains for one run.
func NewHookDispatcher(registry *Registry, audit AuditHandler) *HookDispatcher {
	return &HookDispatcher{
		audit:  audit,
		chains: buildHookChains(registry),
	}
}

// DispatchMutable executes the chain-of-responsibility for one mutable hook.
func (d *HookDispatcher) DispatchMutable(ctx context.Context, hook HookName, input any) (any, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	current := input
	for _, entry := range d.chainEntries(hook) {
		response := executeHookResponse{}
		startedAt := time.Now()
		err := d.invokeHook(ctx, entry, hook, true, current, &response)
		if err != nil {
			d.recordHookAudit(entry, hook, startedAt, err)
			if entry.hook.Required {
				return nil, wrapRequiredHookError(hook, entry.extension.normalizedName(), err)
			}

			slog.Warn(
				"optional hook dispatch failed",
				"component", "extension.dispatcher",
				"extension", entry.extension.normalizedName(),
				"hook", hook,
				"err", err,
			)
			continue
		}

		next, patchErr := applyHookPatch(current, response.Patch)
		if patchErr != nil {
			d.recordHookAudit(entry, hook, startedAt, patchErr)
			if entry.hook.Required {
				return nil, wrapRequiredHookError(hook, entry.extension.normalizedName(), patchErr)
			}

			slog.Warn(
				"optional hook patch rejected",
				"component", "extension.dispatcher",
				"extension", entry.extension.normalizedName(),
				"hook", hook,
				"err", patchErr,
			)
			continue
		}

		d.recordHookAudit(entry, hook, startedAt, nil)
		current = next
	}

	return current, nil
}

// DispatchObserver fans out one observe-only hook to all subscribers using
// best-effort asynchronous delivery.
func (d *HookDispatcher) DispatchObserver(ctx context.Context, hook HookName, payload any) {
	if ctx == nil {
		ctx = context.Background()
	}

	entries := d.chainEntries(hook)
	if len(entries) == 0 {
		return
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		for _, entry := range entries {
			d.recordHookAudit(entry, hook, time.Now(), err)
		}
		slog.Warn(
			"observer hook snapshot failed",
			"component", "extension.dispatcher",
			"hook", hook,
			"err", err,
		)
		return
	}

	workerCount := min(len(entries), max(runtime.GOMAXPROCS(0), 1))
	jobs := make(chan hookChainEntry, len(entries))
	payloadRaw := json.RawMessage(encoded)

	d.pending.Add(len(entries))
	for range workerCount {
		go func(payloadCopy any) {
			for entry := range jobs {
				startedAt := time.Now()
				err := d.invokeHook(ctx, entry, hook, false, payloadCopy, nil)
				d.recordHookAudit(entry, hook, startedAt, err)
				if err != nil {
					slog.Warn(
						"observer hook delivery failed",
						"component", "extension.dispatcher",
						"extension", entry.extension.normalizedName(),
						"hook", hook,
						"err", err,
					)
				}
				d.pending.Done()
			}
		}(payloadRaw)
	}
	for _, entry := range entries {
		jobs <- entry
	}
	close(jobs)
}

func (d *HookDispatcher) waitForObservers() {
	if d == nil {
		return
	}
	d.pending.Wait()
}

func (d *HookDispatcher) chainEntries(hook HookName) []hookChainEntry {
	if d == nil {
		return nil
	}

	return d.chains[HookName(strings.TrimSpace(string(hook)))]
}

func (d *HookDispatcher) invokeHook(
	ctx context.Context,
	entry hookChainEntry,
	hook HookName,
	mutable bool,
	payload any,
	response *executeHookResponse,
) error {
	if entry.extension == nil {
		return fmt.Errorf("hook %q: missing runtime extension", hook)
	}
	if entry.extension.State() != ExtensionStateReady {
		return fmt.Errorf(
			"hook %q extension %q is not ready",
			hook,
			entry.extension.normalizedName(),
		)
	}
	if entry.extension.Caller == nil {
		return fmt.Errorf("hook %q extension %q: missing extension caller", hook, entry.extension.normalizedName())
	}
	if err := entry.extension.Capabilities.CheckHook(hook); err != nil {
		return err
	}

	callCtx := ctx
	cancel := func() {}
	timeout := entry.hook.Timeout
	if timeout <= 0 {
		timeout = entry.extension.DefaultHookTimeout()
	}
	if timeout > 0 {
		callCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()

	request := executeHookRequest{
		InvocationID: d.nextInvocationID(),
		Hook: executeHookRequestHook{
			Name:      effectiveHookName(entry),
			Event:     hook,
			Mutable:   mutable,
			Required:  entry.hook.Required,
			Priority:  entry.hook.Priority,
			TimeoutMS: durationMilliseconds(timeout),
		},
		Payload: payload,
	}

	if response == nil {
		response = &executeHookResponse{}
	}
	return entry.extension.Caller.Call(callCtx, executeHookMethod, request, response)
}

func (d *HookDispatcher) nextInvocationID() string {
	if d == nil {
		return "hook-00000000000000000001"
	}

	value := d.invocation.Add(1)
	return fmt.Sprintf("hook-%020d", value)
}

func (d *HookDispatcher) recordHookAudit(entry hookChainEntry, hook HookName, startedAt time.Time, err error) {
	if d == nil || d.audit == nil || entry.extension == nil {
		return
	}

	recordErr := d.audit.Record(AuditEntry{
		Timestamp:   time.Now().UTC(),
		Extension:   entry.extension.normalizedName(),
		Direction:   AuditDirectionHostToExt,
		Method:      executeHookMethod,
		Capability:  capabilityForHook(hook),
		Latency:     time.Since(startedAt),
		Result:      auditResultForError(err),
		ErrorDetail: auditErrorDetail(err),
	})
	if recordErr != nil {
		slog.Warn(
			"record extension hook audit entry",
			"component", "extension.dispatcher",
			"extension", entry.extension.normalizedName(),
			"hook", hook,
			"err", recordErr,
		)
	}
}

func wrapRequiredHookError(hook HookName, extensionName string, err error) error {
	return fmt.Errorf(
		"dispatch required hook %q via extension %q: %w",
		strings.TrimSpace(string(hook)),
		strings.TrimSpace(extensionName),
		err,
	)
}

func effectiveHookName(entry hookChainEntry) string {
	name := strings.TrimSpace(string(entry.hook.Event))
	if name != "" {
		return name
	}
	return entry.extension.normalizedName()
}

func applyHookPatch(current any, patch json.RawMessage) (any, error) {
	trimmedPatch := bytes.TrimSpace(patch)
	if len(trimmedPatch) == 0 || bytes.Equal(trimmedPatch, []byte("null")) || bytes.Equal(trimmedPatch, []byte("{}")) {
		return current, nil
	}

	var patchFields map[string]json.RawMessage
	if err := json.Unmarshal(trimmedPatch, &patchFields); err != nil {
		return nil, fmt.Errorf("decode hook patch: %w", err)
	}

	currentFields := make(map[string]json.RawMessage)
	if current != nil {
		currentBytes, err := json.Marshal(current)
		if err != nil {
			return nil, fmt.Errorf("marshal hook payload: %w", err)
		}

		trimmedCurrent := bytes.TrimSpace(currentBytes)
		if len(trimmedCurrent) != 0 && !bytes.Equal(trimmedCurrent, []byte("null")) {
			if err := json.Unmarshal(trimmedCurrent, &currentFields); err != nil {
				return nil, fmt.Errorf("decode hook payload: %w", err)
			}
		}
	}

	for key, value := range patchFields {
		currentFields[key] = value
	}

	mergedBytes, err := json.Marshal(currentFields)
	if err != nil {
		return nil, fmt.Errorf("marshal merged hook payload: %w", err)
	}

	return decodeJSONLike(current, mergedBytes)
}

func decodeJSONLike(template any, data []byte) (any, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil, nil
	}

	if template == nil {
		var decoded map[string]any
		if err := json.Unmarshal(trimmed, &decoded); err != nil {
			return nil, fmt.Errorf("decode json value: %w", err)
		}
		return decoded, nil
	}

	valueType := reflect.TypeOf(template)
	if valueType.Kind() == reflect.Interface {
		var decoded any
		if err := json.Unmarshal(trimmed, &decoded); err != nil {
			return nil, fmt.Errorf("decode json interface: %w", err)
		}
		return decoded, nil
	}

	if valueType.Kind() == reflect.Ptr {
		target := reflect.New(valueType.Elem())
		if err := json.Unmarshal(trimmed, target.Interface()); err != nil {
			return nil, fmt.Errorf("decode json pointer: %w", err)
		}
		return target.Interface(), nil
	}

	target := reflect.New(valueType)
	if err := json.Unmarshal(trimmed, target.Interface()); err != nil {
		return nil, fmt.Errorf("decode json value: %w", err)
	}
	return target.Elem().Interface(), nil
}

func auditResultForError(err error) AuditResult {
	if err == nil {
		return AuditResultOK
	}
	return AuditResultError
}

func auditErrorDetail(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func durationMilliseconds(value time.Duration) int64 {
	if value <= 0 {
		return 0
	}

	milliseconds := value / time.Millisecond
	if value%time.Millisecond != 0 {
		milliseconds++
	}
	return int64(milliseconds)
}
