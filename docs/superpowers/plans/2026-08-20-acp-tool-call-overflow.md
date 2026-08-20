# ACP Tool-Call Overflow Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Prevent redundant nonterminal ACP tool-call updates from blocking Compozy's per-prompt event path and overflowing the ACP SDK notification queue, while preserving first-seen state, meaningful enrichment, terminal results, and ordering.

**Architecture:** Keep the fix at `activePromptState`, the narrow owner of ordered per-prompt event delivery. Replace the seen-ID set with a per-tool projection snapshot guarded by the existing `sendMu`; suppress only a seen `EventTypeToolCall` whose non-empty canonical fields add no state, and never suppress results or uncorrelatable empty IDs. Do not change the ACP SDK queue capacity, downstream buffers, public event schema, persistence schema, or configuration.

**Tech Stack:** Go, `coder/acp-go-sdk` v0.13.5, JSON-RPC/stdio ACP subprocess fixture, Go race detector, Compozy QA labs.

**Spec:** [GitHub issue #439](https://github.com/compozy/compozy/issues/439) and `/home/francisross/.codex/handoffs/compozy-acp-overflow-fix.md`.

## Confirmed Reproduction

- Recorded at `2026-08-20T01:57:18Z` in issue #439.
- Trigger: a Codex-backed session ran `rtk node_modules/.bin/biome check .` and emitted a burst of `session/update` notifications for one `toolCallId`.
- Observed evidence: 128 persisted `tool_call` events for one ID in 508 ms; the first carried command metadata and the next 127 carried no canonical state beyond `tool_call_id`.
- Transport failure: `notification queue overflow` at capacity `1024`, followed by `peer disconnected before response` and a failed/stopped/dead session.
- Confirmed path: `handlers_session_state.go` translates synchronously, `handlers_session_update.go` maps nonterminal updates to `EventTypeToolCall`, and `agent_process_prompt.go` synchronously sends each event into the bounded per-prompt channel.
- Root-cause statement: Compozy turns state-equivalent ACP progress notifications into synchronous durable/live events faster than the bounded downstream path can consume them; enlarging the SDK queue changes only the failure threshold.

## Global Constraints

- Keep all work local. Do not push, open a pull request, or mutate issue #439.
- Create an isolated worktree from freshly fetched `upstream/main`; do not implement in the primary checkout.
- Activate `systematic-debugging`, `no-workarounds`, `eng-code-guidelines`, `golang-master`, `eng-consolidate-test-suites`, `eng-test-conventions`, `testing-boss`, and `superpowers:test-driven-development` before source/test edits.
- Activate `deslop` and `cy-final-verify` before completion.
- Use `rtk` for every shell command.
- Follow TDD: observe the focused regression fail on unchanged production code before editing `agent_process_prompt.go`.
- Do not increase ACP SDK or prompt buffer sizes as the production fix.
- Do not add dependencies, configuration keys, migrations, compatibility paths, aliases, or new public fields.
- Never discard an error with `_`.
- Run frontend commands only through Turborepo from the repository root; no frontend command is expected for this backend-only change.
- Run one `make gate-full` only after the final source and QA-doc mutation.

## File Map

- Modify `internal/acp/agent_process_prompt.go`: own the per-prompt tool projection snapshot and suppression decision; keep all state under `sendMu` and scoped to one prompt.
- Modify `internal/acp/types_test.go`: canonical unit regression for bounded delivery, ordering, enrichment, and empty-ID behavior.
- Modify `internal/acp/handlers_test.go`: canonical translation coverage for title/kind/input enrichment and completed/failed terminal results.
- Modify `internal/acp/client_test_support_test.go`: add one helper-agent scenario that emits more than 1024 updates through the real ACP subprocess connection.
- Modify `internal/acp/client_prompt_contract_test.go`: prove the cross-process prompt completes with bounded ordered output.
- Create `docs/qa/scenarios/RT-acp-tool-update-burst.md`: flag and then record the public CLI/runtime/provider walk for issue #439.
- Create/update `docs/qa/reports/2026-08-20-acp-tool-update-burst.md`: durable QA report produced by `qa-execution`.
- No changes to `go.mod`, `go.sum`, `config.toml`, OpenAPI, CLI/HTTP/UDS schemas, Web code, site docs, or `skills/compozy/`.

## Test Placement Decision

- **Invariant:** For one prompt and one non-empty tool-call ID, the first nonterminal tool-call event and every new non-empty canonical projection are delivered once in order, state-equivalent repeats do not consume the bounded event channel, and terminal results are always delivered after the call.
- **Primary owning layer:** unit, because `activePromptState.emitPromptEvent` is the first layer that can prove bounded delivery without reproducing unrelated persistence or transport behavior.
- **Canonical suite:** `internal/acp/types_test.go`.
- **Distinct higher-layer proof:** `internal/acp/client_prompt_contract_test.go` crosses the real `acp-go-sdk` JSON-RPC/stdio subprocess boundary and proves a prompt survives more than 1024 notifications; it does not duplicate the unit test's lock/channel ownership assertion.
- **Translation proof:** `internal/acp/handlers_test.go` owns ACP payload-to-`AgentEvent` mapping and proves the metadata that drives the unit-level projection is not lost.

## Planned Compozy Impact Audit

- **Native tools:** no contract impact; check `internal/tools`, native `compozy__*` descriptors/digests, and capability gates. This change only filters provider-originated ACP lifecycle events before persistence.
- **Extensibility and hooks:** no contract impact; check `internal/hooks`, `internal/extension`, MCP sidecars, bridge SDKs, registries, and config lifecycle. Existing `ToolPrechecked` state remains meaningful and is included in the projection snapshot so hook admission is not lost.
- **Workspace data isolation:** the snapshot is agent-process/prompt-scoped, lives only inside `activePromptState`, is guarded by `sendMu`, and is discarded by `endPrompt`; no global/workspace/session store, cache, SSE key, or event schema is added. Verify that emitted events retain their existing `SessionID`, `TurnID`, and `ToolCallID`.
- **Official Compozy skill:** no impact; no public tool ID, CLI path, hook event, capability, extension resource, memory/network/task semantic, or user instruction changes. Check `skills/compozy/` and leave it unchanged.
- **Web/Docs Impact:** no Web or site-doc change; the public event contract is unchanged and only redundant state-equivalent events are removed. The QA scenario/report are required because prompt survival is user-visible.
- **Config lifecycle:** no impact; `config.toml`, defaults, overlay/merge, settings projection, and config docs expose no tool-update coalescing setting, and this correctness rule must not be configurable.

---

### Task 1: Establish the isolated baseline

**Files:**
- Read: `/home/francisross/.codex/handoffs/compozy-acp-overflow-fix.md`
- Read: `internal/CLAUDE.md`
- Read: `internal/acp/agent_process_prompt.go`
- Read: `internal/acp/types_test.go`
- Read: `internal/acp/handlers_test.go`
- Read: `internal/acp/client_prompt_contract_test.go`
- Read: `internal/acp/client_test_support_test.go`

**Interfaces:**
- Consumes: `upstream/main` after an explicit fetch.
- Produces: clean worktree `/home/francisross/Projects/opensource/_worktrees/acp-overflow-fix` on branch `fix-acp-overflow`, with its base commit recorded in the execution notes.

- [ ] **Step 1: Refresh the official base and verify the relevant diff**

Run from `/home/francisross/Projects/opensource/compozy`:

```bash
rtk git fetch upstream main
rtk git rev-parse upstream/main
rtk git diff --stat origin/main..upstream/main -- internal/acp/agent_process_prompt.go internal/acp/types_test.go internal/acp/handlers_test.go internal/acp/client_prompt_contract_test.go internal/acp/client_test_support_test.go
```

Expected: record the current `upstream/main` SHA. The handoff observed `e8df3c29`; if the SHA changed, re-read the five files and update this plan only if their contracts changed.

- [ ] **Step 2: Create the required worktree**

Run from the primary checkout:

```bash
rtk make worktree-new SLUG=acp-overflow-fix BRANCH=fix-acp-overflow BASE=upstream/main
```

Expected: worktree created at `/home/francisross/Projects/opensource/_worktrees/acp-overflow-fix` and bootstrap completes without modifying the primary checkout.

- [ ] **Step 3: Prove the worktree is clean and based on the fetched commit**

Run with working directory `/home/francisross/Projects/opensource/_worktrees/acp-overflow-fix`:

```bash
rtk git status --short --branch
rtk git merge-base --is-ancestor upstream/main HEAD
rtk git log -1 --oneline --decorate
```

Expected: clean `fix-acp-overflow` branch, the ancestor check exits 0, and HEAD equals the fetched base before edits.

- [ ] **Step 4: Run the unchanged focused package baseline**

```bash
rtk env CGO_ENABLED=1 go test -race ./internal/acp -count=1
```

Expected: PASS. A baseline failure is investigated before any change; do not layer this fix over an unrelated red suite.

---

### Task 2: Add the red regressions and implement prompt-scoped coalescing

**Files:**
- Modify: `internal/acp/types_test.go`
- Modify: `internal/acp/handlers_test.go`
- Modify: `internal/acp/client_test_support_test.go`
- Modify: `internal/acp/client_prompt_contract_test.go`
- Modify: `internal/acp/agent_process_prompt.go`
- Create: `docs/qa/scenarios/RT-acp-tool-update-burst.md`

**Interfaces:**
- Consumes: `AgentProcess.emitPromptEvent(AgentEvent)`, `AgentEvent.Title`, `AgentEvent.ToolName()`, `AgentEvent.ToolKind()`, `AgentEvent.ToolInput()`, `AgentEvent.ToolPrechecked()`, and the existing `sendMu`/`seenToolCalls` lifecycle.
- Produces: unexported `toolCallProjection`, `mergeToolCallProjection(toolCallProjection, AgentEvent) (toolCallProjection, bool)`, and `(*activePromptState).shouldSuppressToolCallLocked(AgentEvent) bool`. No exported or wire interface changes.

- [ ] **Step 1: Add the focused bounded-delivery regression to the canonical suite**

Append this test near the existing `TestEmitPromptEvent...` cases in `internal/acp/types_test.go`:

```go
func TestEmitPromptEventCoalescesRedundantToolCalls(t *testing.T) {
	t.Parallel()

	t.Run("Should keep a repeated tool update burst within the event buffer", func(t *testing.T) {
		t.Parallel()

		const repeatedUpdates = 1025
		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-burst", 2)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}

		emitted := make(chan struct{})
		go func() {
			defer close(emitted)
			proc.emitPromptEvent(AgentEvent{
				Type:       EventTypeToolCall,
				TurnID:     "turn-burst",
				ToolCallID: "tool-burst",
			})
			for range repeatedUpdates {
				proc.emitPromptEvent(AgentEvent{
					Type:       EventTypeToolCall,
					TurnID:     "turn-burst",
					ToolCallID: "tool-burst",
				})
			}
			proc.emitPromptEvent(AgentEvent{
				Type:       EventTypeToolResult,
				TurnID:     "turn-burst",
				ToolCallID: "tool-burst",
			})
		}()

		select {
		case <-emitted:
		case <-time.After(200 * time.Millisecond):
			t.Fatal("tool update burst blocked on the bounded prompt event channel")
		}

		first := <-active.events
		if first.Type != EventTypeToolCall || first.ToolCallID != "tool-burst" {
			t.Fatalf("first event = %#v, want initial tool call", first)
		}
		second := <-active.events
		if second.Type != EventTypeToolResult || second.ToolCallID != "tool-burst" {
			t.Fatalf("second event = %#v, want terminal tool result", second)
		}
		select {
		case event := <-active.events:
			t.Fatalf("unexpected redundant tool event = %#v", event)
		default:
		}
	})
}
```

The goroutine is test-owned, exits by closing `emitted`, and is observed by the test. The timeout is an upper bound on an observable completion condition, not a synchronization sleep.

- [ ] **Step 2: Run the new unit regression and observe the red failure**

```bash
rtk env CGO_ENABLED=1 go test -race ./internal/acp -run 'TestEmitPromptEventCoalescesRedundantToolCalls/Should_keep_a_repeated_tool_update_burst_within_the_event_buffer' -count=1
```

Expected before production edits: FAIL with `tool update burst blocked on the bounded prompt event channel`. If it passes, stop and re-check the reproduction because the planned seam is no longer valid.

- [ ] **Step 3: Add projection and correlation boundary cases before implementation**

Add two more subtests inside `TestEmitPromptEventCoalescesRedundantToolCalls`:

```go
	t.Run("Should emit each new tool projection and the terminal result once", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-enrichment", 6)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}

		base := AgentEvent{Type: EventTypeToolCall, TurnID: "turn-enrichment", ToolCallID: "tool-enrichment"}
		proc.emitPromptEvent(base)
		proc.emitPromptEvent(base)
		proc.emitPromptEvent((AgentEvent{
			Type: EventTypeToolCall, TurnID: "turn-enrichment", ToolCallID: "tool-enrichment", Title: "Read file",
		}).WithTool("Read file", nil, false))
		proc.emitPromptEvent((AgentEvent{
			Type: EventTypeToolCall, TurnID: "turn-enrichment", ToolCallID: "tool-enrichment", Title: "Read file",
		}).WithTool("Read file", nil, false))
		proc.emitPromptEvent(base.WithToolKind("read"))
		proc.emitPromptEvent(base.WithTool("", json.RawMessage(`{"path":"README.md"}`), false))
		proc.emitPromptEvent(base.WithToolPrechecked())
		proc.emitPromptEvent(AgentEvent{
			Type: EventTypeToolResult, TurnID: "turn-enrichment", ToolCallID: "tool-enrichment",
		})

		events := collectEventsUntilCount(t, active.events, 6)
		if events[0].Type != EventTypeToolCall || events[0].HasToolPayload() {
			t.Fatalf("initial event = %#v, want sparse tool call", events[0])
		}
		if events[1].Title != "Read file" || events[1].ToolName() != "Read file" {
			t.Fatalf("title event = %#v, want title enrichment", events[1])
		}
		if events[2].ToolKind() != "read" {
			t.Fatalf("kind event = %#v, want read enrichment", events[2])
		}
		if got, want := string(events[3].ToolInput()), `{"path":"README.md"}`; got != want {
			t.Fatalf("input event = %s, want %s", got, want)
		}
		if !events[4].ToolPrechecked() {
			t.Fatalf("prechecked event = %#v, want preserved admission state", events[4])
		}
		if events[5].Type != EventTypeToolResult {
			t.Fatalf("terminal event = %#v, want tool result", events[5])
		}
	})

	t.Run("Should preserve tool calls without a correlation id", func(t *testing.T) {
		t.Parallel()

		proc := &AgentProcess{}
		active, err := proc.beginPrompt("turn-no-id", 2)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}
		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolCall, TurnID: "turn-no-id"})
		proc.emitPromptEvent(AgentEvent{Type: EventTypeToolCall, TurnID: "turn-no-id"})

		for index := range 2 {
			event := <-active.events
			if event.Type != EventTypeToolCall {
				t.Fatalf("event %d = %#v, want uncorrelated tool call", index, event)
			}
		}
	})
```

The enrichment subtest must also fail before the fix because identical state is still delivered; the empty-ID case must remain green and protects against over-broad suppression.

- [ ] **Step 4: Add the ACP translation preservation subtest**

Add a sibling case inside `TestHandleSessionUpdateVariants` in `internal/acp/handlers_test.go`. Send, in order, a sparse update, title enrichment, kind enrichment, raw-input enrichment, completed result, a second sparse call, and failed result:

```go
	t.Run("Should preserve tool enrichment and both terminal statuses", func(t *testing.T) {
		t.Parallel()

		proc := newDirectProcess(t, compozyconfig.PermissionModeApproveAll)
		active, err := proc.beginPrompt("turn-tool-enrichment", 7)
		if err != nil {
			t.Fatalf("beginPrompt() error = %v", err)
		}
		defer proc.endPrompt(active)

		updates := []acpsdk.SessionUpdate{
			acpsdk.UpdateToolCall("tool-completed"),
			acpsdk.UpdateToolCall("tool-completed", acpsdk.WithUpdateTitle("Read file")),
			acpsdk.UpdateToolCall("tool-completed", acpsdk.WithUpdateKind(acpsdk.ToolKindRead)),
			acpsdk.UpdateToolCall("tool-completed", acpsdk.WithUpdateRawInput(map[string]any{"path": "README.md"})),
			acpsdk.UpdateToolCall("tool-completed", acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusCompleted)),
			acpsdk.UpdateToolCall("tool-failed"),
			acpsdk.UpdateToolCall("tool-failed", acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusFailed)),
		}
		for _, update := range updates {
			raw := mustMarshalJSON(acpsdk.SessionNotification{SessionId: "sess-direct", Update: update})
			if err := proc.handleSessionUpdate(raw); err != nil {
				t.Fatalf("handleSessionUpdate() error = %v", err)
			}
		}

		events := collectEventsUntilCount(t, active.events, len(updates))
		if events[0].HasToolPayload() {
			t.Fatalf("initial event = %#v, want sparse tool call", events[0])
		}
		if events[1].Title != "Read file" || events[1].ToolName() != "Read file" {
			t.Fatalf("title event = %#v, want title enrichment", events[1])
		}
		if events[2].ToolKind() != "read" {
			t.Fatalf("kind event = %#v, want read enrichment", events[2])
		}
		if got, want := string(events[3].ToolInput()), `{"path":"README.md"}`; got != want {
			t.Fatalf("input event = %s, want %s", got, want)
		}
		if events[4].Type != EventTypeToolResult || events[4].ToolError() {
			t.Fatalf("completed event = %#v, want successful tool result", events[4])
		}
		if events[6].Type != EventTypeToolResult || !events[6].ToolError() {
			t.Fatalf("failed event = %#v, want failed tool result", events[6])
		}
	})
```

This test owns translation only. Do not add persistence, Web, transcript, or API assertions for the same invariant.

- [ ] **Step 5: Add the cross-process burst fixture and prompt contract**

Add this case to `helperACPAgent.Prompt` in `internal/acp/client_test_support_test.go`:

```go
	case "tool_update_burst":
		const repeatedUpdates = 1100
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.StartToolCall(
				"tool-burst",
				"Run command",
				acpsdk.WithStartKind(acpsdk.ToolKindExecute),
				acpsdk.WithStartStatus(acpsdk.ToolCallStatusInProgress),
			),
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
		for range repeatedUpdates {
			if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
				SessionId: params.SessionId,
				Update: acpsdk.UpdateToolCall(
					"tool-burst",
					acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusInProgress),
				),
			}); err != nil {
				return acpsdk.PromptResponse{}, err
			}
		}
		if err := a.conn.SessionUpdate(ctx, acpsdk.SessionNotification{
			SessionId: params.SessionId,
			Update: acpsdk.UpdateToolCall(
				"tool-burst",
				acpsdk.WithUpdateStatus(acpsdk.ToolCallStatusCompleted),
			),
		}); err != nil {
			return acpsdk.PromptResponse{}, err
		}
```

Add this sibling case to `TestPromptStreamsSessionUpdates` in `internal/acp/client_prompt_contract_test.go`:

```go
	t.Run("Should complete after more than 1024 redundant tool updates", func(t *testing.T) {
		t.Parallel()

		driver := New(WithPromptBufferSize(2))
		proc := startHelperProcess(t, driver, "tool_update_burst", "", StartOpts{})
		defer stopProcess(t, driver, proc)

		eventsCh, err := driver.Prompt(testutil.Context(t), proc, PromptRequest{
			TurnID:  "turn-tool-burst",
			Message: "run burst",
		})
		if err != nil {
			t.Fatalf("Prompt() error = %v", err)
		}

		events := collectEvents(t, eventsCh)
		gotTypes := make([]string, 0, len(events))
		for _, event := range events {
			gotTypes = append(gotTypes, event.Type)
		}
		wantTypes := []string{EventTypeToolCall, EventTypeToolResult, EventTypeDone}
		if !slices.Equal(gotTypes, wantTypes) {
			t.Fatalf("Prompt() event types = %#v, want %#v", gotTypes, wantTypes)
		}
		if events[0].ToolCallID != "tool-burst" || events[1].ToolCallID != "tool-burst" {
			t.Fatalf("Prompt() tool lifecycle = %#v, want tool-burst", events[:2])
		}
	})
```

- [ ] **Step 6: Observe the cross-process contract fail before production edits**

```bash
rtk env CGO_ENABLED=1 go test -race ./internal/acp -run 'TestPromptStreamsSessionUpdates/Should_complete_after_more_than_1024_redundant_tool_updates' -count=1
```

Expected before the fix: FAIL with extra `tool_call` events or an ACP disconnect/error. The test must not pass with the unchanged event fan-out.

- [ ] **Step 7: Implement the smallest responsible-layer change**

In `internal/acp/agent_process_prompt.go`, replace `seenToolCalls map[string]struct{}` with the following unexported projection:

```go
type toolCallProjection struct {
	title       string
	name        string
	kind        string
	inputDigest [sha256.Size]byte
	hasInput    bool
	prechecked  bool
}

type activePromptState struct {
	// existing fields unchanged
	seenToolCalls        map[string]toolCallProjection
	pendingToolResults   []AgentEvent
	pendingToolResultIDs map[string]struct{}
	// existing fields unchanged
}
```

Initialize it in `beginPrompt`:

```go
		seenToolCalls:        make(map[string]toolCallProjection),
```

Add these focused helpers in the same file near `deferToolResultLocked`:

```go
func (a *activePromptState) shouldSuppressToolCallLocked(event AgentEvent) bool {
	if a == nil || event.Type != EventTypeToolCall {
		return false
	}
	toolCallID := strings.TrimSpace(event.ToolCallID)
	if toolCallID == "" {
		return false
	}

	current, seen := a.seenToolCalls[toolCallID]
	next, changed := mergeToolCallProjection(current, event)
	if !seen {
		a.seenToolCalls[toolCallID] = next
		return false
	}
	if !changed {
		return true
	}
	a.seenToolCalls[toolCallID] = next
	return false
}

func mergeToolCallProjection(current toolCallProjection, event AgentEvent) (toolCallProjection, bool) {
	changed := false
	if title := strings.TrimSpace(event.Title); title != "" && title != current.title {
		current.title = title
		changed = true
	}
	if name := strings.TrimSpace(event.ToolName()); name != "" && name != current.name {
		current.name = name
		changed = true
	}
	if kind := strings.TrimSpace(event.ToolKind()); kind != "" && kind != current.kind {
		current.kind = kind
		changed = true
	}
	if input := event.ToolInput(); len(input) > 0 {
		digest := sha256.Sum256(input)
		if !current.hasInput || digest != current.inputDigest {
			current.inputDigest = digest
			current.hasInput = true
			changed = true
		}
	}
	if event.ToolPrechecked() && !current.prechecked {
		current.prechecked = true
		changed = true
	}
	return current, changed
}
```

Add `crypto/sha256` to the file's imports. The digest keeps retained input state fixed-size while still distinguishing changed canonical input.

Call suppression after deferred-result handling and before any channel send:

```go
	if active.deferToolResultLocked(event) {
		return
	}
	if active.shouldSuppressToolCallLocked(event) {
		return
	}
	if event.Type == EventTypeDone {
		active.flushDeferredToolResultsLocked()
	}
	active.sendEventLocked(event)
	if event.Type == EventTypeToolCall {
		active.flushDeferredToolResultsForToolLocked(event.ToolCallID)
	}
```

Delete `markToolCallSeenLocked`; `shouldSuppressToolCallLocked` now records the first correlated call before delivery. Keep terminal results outside suppression. Ignore `Raw`, progress content, locations, and raw output because those fields are not canonical/projected tool state in `AgentEvent` and are the source of the amplification.

- [ ] **Step 8: Run the red regressions again and require green**

```bash
rtk env CGO_ENABLED=1 go test -race ./internal/acp -run 'TestEmitPromptEventCoalescesRedundantToolCalls|TestHandleSessionUpdateVariants|TestPromptStreamsSessionUpdates' -count=1
```

Expected: PASS. Exact lifecycle output for the burst is `tool_call`, `tool_result`, `done`; the sparse first event, enrichment events, successful/failed results, and empty-ID events remain observable.

- [ ] **Step 9: Run the Go test convention checker and the entire ACP package**

```bash
rtk python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/acp/types_test.go
rtk python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/acp/handlers_test.go
rtk python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/acp/client_prompt_contract_test.go
rtk python3 .agents/skills/eng/eng-test-conventions/scripts/check-test-conventions.py internal/acp/client_test_support_test.go
rtk env CGO_ENABLED=1 go test -race ./internal/acp -count=1
```

Expected: all convention checks and package tests PASS with no race.

- [ ] **Step 10: Create the QA impact flag before closing implementation**

Create `docs/qa/scenarios/RT-acp-tool-update-burst.md` with this content:

```markdown
---
id: RT-acp-tool-update-burst
area: RT
title: Keep an ACP prompt healthy during a tool update burst
persona: Théo
journey: J-13
expected: A Codex-backed prompt that produces a large burst of nonterminal updates for one tool call completes without `notification queue overflow` or `peer disconnected before response`; the public event stream contains the first call, meaningful metadata enrichment, one terminal result, and `done` in order without state-equivalent duplicates.
entry_points: compozy session prompt -o jsonl; session events; daemon logs
qa_status: untested
bug_ids: 439
fix_status: fixed
retest_status:
fix_commits:
evidence:
last_report:
overlaps: RT-018; RT-acp-stream-disconnect-recovery
---

Issue #439 is distinct from issue #315: the provider remains healthy until redundant
`session/update` notifications fill the ACP SDK queue. This scenario owns prompt survival and the
public tool lifecycle under a single-call update burst; RT-acp-stream-disconnect-recovery continues
to own recovery after a provider process actually disconnects.

Walk the scenario through an isolated native Codex session. Trigger one command with sustained
streaming output, capture CLI JSONL plus the durable session events and daemon log, and verify the
same tool-call ID has an ordered initial call and terminal result, the prompt reaches `done`, and
neither overflow nor peer-disconnect diagnostics occur.
```

- [ ] **Step 11: Run the scoped gate and inspect the diff**

```bash
rtk make gate
rtk git diff --check
rtk git diff --stat
rtk git status --short
```

Expected: scoped Go lane PASS, no whitespace errors, only the six planned files changed, and the QA scenario is still `untested` pending Task 3.

- [ ] **Step 12: Commit the implementation batch locally**

```bash
rtk git add internal/acp/agent_process_prompt.go internal/acp/types_test.go internal/acp/handlers_test.go internal/acp/client_test_support_test.go internal/acp/client_prompt_contract_test.go docs/qa/scenarios/RT-acp-tool-update-burst.md
rtk git commit -m "fix: coalesce redundant ACP tool updates"
```

Expected: one local commit. Do not push it.

---

### Task 3: Walk the public behavior and close the QA tracker

**Files:**
- Modify: `docs/qa/scenarios/RT-acp-tool-update-burst.md`
- Create: `docs/qa/reports/2026-08-20-acp-tool-update-burst.md`
- Create under lab scratch only: evidence paths referenced by the QA report.

**Interfaces:**
- Consumes: built local Compozy binary, CLI JSONL session prompt/events, native Codex provider using the manifest's operator-home policy, isolated daemon/logs.
- Produces: a `pass`, `fail`, or explicitly blocked scenario verdict; report and evidence paths; clean teardown evidence with `"clean": true`.

- [ ] **Step 1: Build the implementation and bootstrap a targeted isolated lab**

Run from the isolated worktree:

```bash
rtk make build
rtk python3 .agents/skills/eng/eng-qa-bootstrap/scripts/bootstrap-qa-env.py --scenario "acp-tool-update-burst" --repo-root . --profile targeted --required-surface "cli" --required-surface "runtime" --required-surface "provider"
```

Expected: one fresh `BOOTSTRAP_MANIFEST`. Read it; export only its exact environment values, preserve operator `HOME` for `native_cli + home_policy=operator`, and use its `COMPOZY_HOME`, ports, evidence path, audit command, and teardown command.

- [ ] **Step 2: Start the manifest-owned runtime and register every process**

Follow `.agents/skills/eng/eng-qa-bootstrap/references/bootstrap-contract.md` and the manifest commands exactly. Register daemon/watch processes at `<QA_OUTPUT_PATH>/qa/pids/<name>.pid` immediately after spawn. Do not use the default Compozy home or port.

Expected: isolated daemon reachable through the manifest's CLI/UDS surface; no unrelated lab or operator daemon is touched.

- [ ] **Step 3: Walk the burst through the public CLI as Théo**

Create a native Codex-backed session in the manifest workspace and prompt it through `compozy session prompt ... -o jsonl` to run exactly:

```text
rtk sh -c 'for i in $(seq 1 200000); do printf "burst-%06d\n" "$i"; done'
```

Capture:

- the prompt JSONL stream;
- `compozy session events <session-id> -o jsonl` after completion;
- session status/inspect output;
- the isolated daemon log for the prompt time window.

Expected public observations:

- prompt ends with `done`, not `error`;
- session remains attachable/healthy rather than failed/stopped/dead from transport overflow;
- one tool-call ID has its initial call before its completed/failed terminal result;
- no state-equivalent public `tool_call` flood remains for that ID;
- daemon log contains neither `notification queue overflow` nor `peer disconnected before response`.

If the provider aggregates the command so heavily that it does not create a meaningful update burst, record `blocked-verify` with that exact limitation; do not claim a pass from absence of an overflow trigger. The automated 1100-update subprocess contract remains the deterministic proof.

- [ ] **Step 4: Verify through an independent read path and record the report**

Use session events/status as the independent read path after the CLI stream finishes. Create `docs/qa/reports/2026-08-20-acp-tool-update-burst.md` from `docs/qa/templates/report.md`, record the single-scenario matrix and evidence, and update `docs/qa/scenarios/RT-acp-tool-update-burst.md`:

- `qa_status: pass` and `retest_status: pass` only when the burst was actually observed and all expectations passed;
- otherwise `qa_status: blocked-verify` with the precise missing trigger/evidence;
- set `fix_commits` to the local implementation commit SHA;
- set `evidence` and `last_report` to the durable report/evidence paths.

- [ ] **Step 5: Run the strict audit and mandatory teardown on every outcome**

Run the manifest's exact `AUDIT_COMMAND`, then always run:

```bash
eval "$TEARDOWN_COMMAND"
```

Expected: `<QA_OUTPUT_PATH>/qa/teardown.json` exists and contains `"clean": true`. PASS, FAIL, BLOCKED, and abort all require the same teardown; no daemon, tmux server, browser, watcher, or provider subprocess may survive.

- [ ] **Step 6: Commit the QA evidence locally**

```bash
rtk git add docs/qa/scenarios/RT-acp-tool-update-burst.md docs/qa/reports/2026-08-20-acp-tool-update-burst.md
rtk git commit -m "test: record ACP tool update burst QA"
```

Expected: one local QA commit. Do not push it.

---

### Task 4: Audit scope, deslop, and run the single full gate

**Files:**
- Review only: all files changed against `upstream/main`.
- No mutation after the full gate unless the gate is intentionally rerun because the fingerprint became stale.

**Interfaces:**
- Consumes: completed implementation and QA commits.
- Produces: clean diff, completed Compozy Impact Audit, fresh full-gate record, exact local evidence for user review.

- [ ] **Step 1: Run the required deslop review**

Activate `deslop`, inspect:

```bash
rtk git diff upstream/main...HEAD -- internal/acp docs/qa/scenarios/RT-acp-tool-update-burst.md docs/qa/reports/2026-08-20-acp-tool-update-burst.md
rtk git diff --check upstream/main...HEAD
rtk wc -l internal/acp/agent_process_prompt.go
```

Expected: no unrelated refactor, queue-size workaround, repeated comments, compatibility code, ignored error, or production file over 500 lines. If deslop changes content, rerun Task 2's focused race suite and `make gate` before continuing.

- [ ] **Step 2: Complete the Compozy Impact Audit with checked evidence**

Record in completion notes:

```markdown
Compozy Impact Audit:

- Native tools: no impact — checked `internal/tools` native IDs/descriptors/digests and capability gates; no native tool contract changed.
- Extensibility and hooks: no impact — checked `internal/hooks`, `internal/extension`, MCP/bridge registries, and config lifecycle; `ToolPrechecked` remains part of meaningful per-call state and no hook/resource/config surface changed.
- Workspace data isolation: no impact — the new snapshot is agent-process/prompt-scoped under `activePromptState.sendMu`, keyed only by the current prompt's `toolCallId`, discarded at prompt end, and never enters global/workspace/session stores or caches; existing event correlation is unchanged.
- Official Compozy skill: no impact — checked `skills/compozy/`; no public tool ID, CLI path, hook event, capability, extension resource, or memory/network/task behavior changed.
```

Also record: `Web/Docs Impact: no web/site impact; public event shape and management surfaces are unchanged. QA tracker/report updated for the reliability behavior.`

- [ ] **Step 3: Run final verification exactly once after source freeze**

Activate `cy-final-verify`, then run:

```bash
rtk make gate-full
rtk make gate-status
rtk git status --short --branch
rtk git log --oneline --decorate -3
```

Expected: full gate PASS with a current fingerprint, worktree clean, two local commits above the recorded `upstream/main` base, zero warnings/errors, and no QA process survivors.

- [ ] **Step 4: Hand off exact local evidence without external mutation**

Report to the user:

- worktree path and branch;
- base SHA and both local commit SHAs;
- red-before failure text;
- focused race commands and PASS results;
- cross-process 1100-update event sequence;
- QA verdict/report/evidence and `teardown.json` with `"clean": true`;
- `make gate-status` full-gate fingerprint/result;
- completed Compozy Impact Audit;
- explicit confirmation that nothing was pushed and no PR was opened.

Do not comment on issue #439, push the branch, or open a pull request.
