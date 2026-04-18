---
status: resolved
file: internal/api/core/internal_helpers_test.go
line: 235
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc579yyB,comment:PRRC_kwDORy7nkc65HZWR
---

# Issue 004: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Cover every terminal event that stops the stream.**

`isTerminalRunEvent` is part of the `StreamRun` stop condition, but this test only exercises `run.completed` and one non-terminal kind. A regression in `run.failed`, `run.cancelled`, or the shutdown terminal kinds would slip through even though it changes live-stream behavior.

<details>
<summary>Suggested table-driven coverage</summary>

```diff
 func TestCursorHelpersAndTerminalEvents(t *testing.T) {
 	timestamp := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
 	event := events.Event{
 		Seq:       2,
 		Timestamp: timestamp,
 		Kind:      events.EventKindRunCompleted,
 	}
@@
-	if !isTerminalRunEvent(events.EventKindRunCompleted) {
-		t.Fatal("isTerminalRunEvent(run.completed) = false, want true")
-	}
-	if isTerminalRunEvent(events.EventKindSessionUpdate) {
-		t.Fatal("isTerminalRunEvent(session.update) = true, want false")
-	}
+	testCases := []struct {
+		name string
+		kind events.EventKind
+		want bool
+	}{
+		{"Should mark run.completed as terminal", events.EventKindRunCompleted, true},
+		{"Should mark run.failed as terminal", events.EventKindRunFailed, true},
+		{"Should mark run.cancelled as terminal", events.EventKindRunCancelled, true},
+		{"Should mark shutdown.requested as terminal", events.EventKindShutdownRequested, true},
+		{"Should mark shutdown.draining as terminal", events.EventKindShutdownDraining, true},
+		{"Should mark shutdown.terminated as terminal", events.EventKindShutdownTerminated, true},
+		{"Should leave session.update non-terminal", events.EventKindSessionUpdate, false},
+	}
+	for _, tc := range testCases {
+		t.Run(tc.name, func(t *testing.T) {
+			t.Parallel()
+			if got := isTerminalRunEvent(tc.kind); got != tc.want {
+				t.Fatalf("isTerminalRunEvent(%s) = %v, want %v", tc.kind, got, tc.want)
+			}
+		})
+	}
 }
```
</details>

As per coding guidelines, "Focus on critical paths: workflow execution, state management, error handling" and "MUST test meaningful business logic, not trivial operations".

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
func TestCursorHelpersAndTerminalEvents(t *testing.T) {
	timestamp := time.Date(2026, 4, 17, 14, 0, 0, 0, time.UTC)
	event := events.Event{
		Seq:       2,
		Timestamp: timestamp,
		Kind:      events.EventKindRunCompleted,
	}

	cursor := CursorFromEvent(event)
	if cursor.Sequence != 2 || !cursor.Timestamp.Equal(timestamp) {
		t.Fatalf("CursorFromEvent() = %#v, want timestamp=%s sequence=2", cursor, timestamp)
	}

	if !EventAfterCursor(event, StreamCursor{Timestamp: timestamp, Sequence: 1}) {
		t.Fatal("EventAfterCursor(after older cursor) = false, want true")
	}
	if !EventAfterCursor(event, StreamCursor{}) {
		t.Fatal("EventAfterCursor(zero cursor) = false, want true")
	}
	if EventAfterCursor(event, StreamCursor{Timestamp: timestamp, Sequence: 2}) {
		t.Fatal("EventAfterCursor(equal cursor) = true, want false")
	}
	if EventAfterCursor(event, StreamCursor{Timestamp: timestamp.Add(time.Second), Sequence: 1}) {
		t.Fatal("EventAfterCursor(older event) = true, want false")
	}

	testCases := []struct {
		name string
		kind events.EventKind
		want bool
	}{
		{"Should mark run.completed as terminal", events.EventKindRunCompleted, true},
		{"Should mark run.failed as terminal", events.EventKindRunFailed, true},
		{"Should mark run.cancelled as terminal", events.EventKindRunCancelled, true},
		{"Should mark shutdown.requested as terminal", events.EventKindShutdownRequested, true},
		{"Should mark shutdown.draining as terminal", events.EventKindShutdownDraining, true},
		{"Should mark shutdown.terminated as terminal", events.EventKindShutdownTerminated, true},
		{"Should leave session.update non-terminal", events.EventKindSessionUpdate, false},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := isTerminalRunEvent(tc.kind); got != tc.want {
				t.Fatalf("isTerminalRunEvent(%s) = %v, want %v", tc.kind, got, tc.want)
			}
		})
	}
}
```

</details>

<!-- suggestion_end -->

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/api/core/internal_helpers_test.go` around lines 204 - 235, The test
TestCursorHelpersAndTerminalEvents only asserts run.completed and one
non-terminal kind; extend it to cover every event kind that should stop a
StreamRun by adding table-driven cases that assert isTerminalRunEvent returns
true for events.EventKindRunCompleted, events.EventKindRunFailed,
events.EventKindRunCancelled and any shutdown/termination kinds your code treats
as terminal, and false for several non-terminal kinds (e.g.,
events.EventKindSessionUpdate); update the test loop in internal_helpers_test.go
to iterate the table and call isTerminalRunEvent for each case so regressions in
isTerminalRunEvent are caught.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:34d8a7c1-5aaf-4e7e-85b1-924950384777 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Root cause: `isTerminalRunEvent` currently drives `StreamRun` termination for six terminal event kinds, but the test only covers `run.completed` and one non-terminal event, so regressions in the other stop conditions would go undetected.
- Fix plan: replace the ad-hoc assertions with a table-driven test that covers every terminal kind plus representative non-terminal coverage.
- Resolution: `internal/api/core/internal_helpers_test.go` now uses a table-driven `Should...` matrix for all terminal kinds plus representative non-terminal cases.
