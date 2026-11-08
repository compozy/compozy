# Instance attention acknowledgement

Issue: #1393. Scenario: `MS-web-stays-on-reports`.

The old mark-seen implementation advanced the operator watermark and reset unattended turns,
while the avatar's attention query still counted every pending failure report and unresolved
interaction in the instance lineage. Reading the instance therefore could not clear its attention.

Mark as seen now acknowledges failure attention through the existing local-operator watermark,
without modifying provider report delivery state. It cancels stopped-lineage interactions through
the canonical pending-interaction transition, including session counts and attention revisions.
Live requests remain answerable and keep attention. The additive `attention_dismissible` field
controls whether the Web offers the action. HTTP/UDS, CLI and native tools share the same API owner;
operator-only guards remain in place. A catalog event and mutation invalidation refresh avatars.

## Focused evidence

- `TestInstanceSeen`: real SQLite; old failures clear, live requests survive, stopped requests are
  canceled with counts/revisions updated, report delivery state stays pending, and newer failures
  raise attention again. Existing monotonic watermark and unattended-pause cases remain intact.
- `TestInstanceCallerLineage`: acknowledgement reaches an orphaned stopped descendant.
- `TestAgentInstanceAway`: production HTTP handler with real SQLite; passive reads stay passive,
  acknowledgement clears attention and publishes exactly one instance update.
- `TestNativeInstanceAuthority`: existing operator/agent authority checks.
- `TestManagerAttentionHookRunsAfterCanonicalCommitAndFailsOpen`: external interaction cancellation
  clears resident session attention and emits one normal hook edge per session for a batch, including
  fail-open observers.
- Web row suite: hover-card and context-menu actions; live requests do not offer Mark as seen.
  Hook suite: server reread clears cached attention in the instance's profile; explicit 404 rejects.
- Root Turbo focused tests and Web typecheck, codegen/check, scoped Go lint, focused Oxlint and
  React Doctor (100/100). CI owns repository-wide and race/E2E lanes.

## Rendered flow

A temporary isolated HTTP harness used the production instance list/mark-seen handlers and real
SQLite repositories, with a seeded failure report. A Vite page composed the production
`AlwaysOnRow`, query options, adapter and mutation. The browser observed the avatar's accessible
name change from `Triage · Global · Needs you` to `Triage · Global · Idle` without page reload;
the server reread confirmed `needs_attention: false`. The action also disappeared.

The browser's CDP pointer/key dispatch did not trigger the action reliably; the recorded mutation
used the rendered button's DOM click. Component tests separately exercise user events and the
context-menu path. This is a rendered component/HTTP integration check, not a full desktop or
live-provider session journey. No local E2E suite ran and the user's daemon was untouched.

| Before acknowledgement | After acknowledgement |
| --- | --- |
| ![Avatar with orange attention triangle](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1393/before.png) | ![Avatar after attention cleared](https://pub-6cb601c36b3b4de9bc57fae6b15686d0.r2.dev/issue-1393/after.png) |

Both captures use 1440 × 900. Small PNG sizes reflect the deliberately sparse component surface;
both were visually inspected. Remote evidence URLs returned HTTP 200 with `image/png`.
The targeted lab teardown completed with `clean: true`; temporary harness sources were removed.

## Review dispositions

Claude Opus 5.5 (medium), plain read-only UI review, identified the action availability, copy,
404 handling, durable error feedback and alternate context-menu access. These were repaired.
The follow-up review confirmed those dispositions and identified missing session projection
bookkeeping; canonical transitions and the session publication path now own that work. A final
confirmation identified duplicate hook edges for multiple cancellations in one session; publication
now groups by session, refreshes its pending records and emits only one edge.

Leaving the conversation continues to count as seen, as the existing hook already specifies.
The native action still rejects non-operator callers in `authorizeNativeInstanceAction`; tool
visibility alone does not confer operator authority. Provider delivery and acknowledgement are
separate: reports remain available to the agent after the operator reads them.

Greptile review `discussion_r4507284015` identified request cancellation after the durable
acknowledgement. Publication now detaches its reads from caller cancellation and bounds them with
the existing lifecycle timeout. The canonical hook suite cancels the request before publication,
requires a deadline at the storage read, and verifies the attention edge and resident state still clear.

CodeRabbit review `5819097846` / thread `discussion_r4507306377` requested transient-read retry;
publication now makes the same bounded two-attempt retry used by the nearby permission lifecycle.
The hook suite injects a failed first read and verifies recovery. Both embedded test nitpicks were
applied: individual live interaction statuses remain unchanged, and the non-dismissible menu test
first proves an existing action is visible. The two new public query/API methods now explain their
non-obvious acknowledgement boundaries in one-line comments.

CodeRabbit review `5819118895` / thread `discussion_r4507325904` requested an independent budget
for each session. Each refresh now gets its own detached lifecycle timeout; a timed-out earlier
session cannot consume a later session's deadline. The canonical session suite exercises that
failure and preserves a newer pending interaction returned with current attention from the store
(embedded nitpick `cr-comment:v1:9a0da8c64b51765e3c9c375f`).
