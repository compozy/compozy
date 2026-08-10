---
title: Rewind a session to an earlier checkpoint
type: feature
---

You can now rewind an idle session back to one of your earlier messages instead of starting over. The selected message and everything after it leave the active transcript, the message text comes back as a composer draft, and the session continues under the same session ID with a fresh agent context rebuilt only from the part you kept. Rewind touches the conversation only — it does not undo file edits, tool effects, network activity, saved memory, or anything the provider already did outside CompozyOS — and the discarded events stay archived for audit. (#310)

- `compozy session rewind <session-id>` picks the cut point with `--message-id` and reads the current transcript fences for you; scripts retrying a known request pass `--expected-generation`, `--expected-epoch`, and `--expected-max-sequence` together with the original `--idempotency-key`. Agents get `compozy__session_rewind`.
- Retrying the same rewind with the same idempotency key returns the original result, and the response carries the `draft_text` that goes back into the composer.
- A rewind is refused with a clear conflict — and without cutting the transcript — when the fence is stale, the session is busy, input is queued, an approval is pending, or the session is daemon-managed. It is serialized against clear, delete, repair, resume, and other prompt-producing operations.
- In the web UI, the action appears on your own durable messages, requires an empty composer, confirms the side-effect boundary before it runs, and restores the draft afterward.
- Reads take an `archive` selector: `compozy session events` and `compozy session history` accept `--archive active|archived|all`, and the same selector exists on the HTTP and UDS reads.

Migration notes: `session events` and `session history` now default to `archive=active`. They previously returned archived rows alongside active ones — pass `--archive all` to keep the old behavior.
