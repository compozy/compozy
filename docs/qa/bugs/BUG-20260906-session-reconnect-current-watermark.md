# BUG-20260906-session-reconnect-current-watermark: reconnect never confirms an already current transcript

- **Status:** fixed — live recovery re-walk passed
- **Impact:** Confusion · **Severity:** Major · **Priority:** P1
- **Persona:** Théo · **Scenario:** RT-visible-session-streaming
- **Found:** 2026-09-06 · **Report:** docs/qa/reports/2026-09-06-sessions-stability.md

After retrying the current transcript cursor, EventSource opens but the Web remains Connecting
until another transcript event arrives. The server previously wrote no initial body when
TranscriptChanges returned an empty page without cursor advance. Vite also buffered the bare
headers until the first keepalive. Reproduction compared the same durable cursor against the
native and proxied endpoints; the idle server returned no transcript confirmation.

The shared initializer now sends an empty existing transcript_delta frame with the current
cursor and fences, max_sequence and has_more=false. It does not invent a message or advance
state. TestInitializeTranscriptStream's current-watermark case failed with an empty body before
the change and passed afterward. Focused race suites, test-shape check and stream lint pass.
Integrated evidence/transport-empty-head-native.sse and the localhost proxy capture show the
same empty delta at cursor 4651, epoch 0, generation 1. No field or schema migration is needed.

Actual browser Try again recovered within the 500ms observation at unchanged cursor 4651,
epoch0/generation1, EventSource OPEN. The degraded header/marker disappeared, the exact draft
remained unsent and errors=[] (transport-fixed-recovered.json/png, PNG inspected). A fresh
public prompt after the owned daemon restart also proves the persisted session remains usable.
