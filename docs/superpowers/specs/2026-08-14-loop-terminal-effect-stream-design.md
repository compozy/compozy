# Terminal Loop Effect Stream Lifecycle Design

**Issue:** [compozy/compozy#405](https://github.com/compozy/compozy/issues/405)

## Goal

Keep the Loop run event subscription alive for the mounted run-details page so terminal
`effect_results` frames are delivered after the run's terminal `status_changed` frame. Preserve
the existing workspace/run identity fence, replay cursor, named-event routing, stale-subscription
protection, error forwarding, query invalidation, and cleanup behavior.

## Confirmed Reproduction

On 2026-08-14, an isolated Compozy `v0.3.0-beta.16` workspace ran a minimal Loop with an
`on_done` tool effect. The retained API event stream contained this ordered sequence:

```text
status_changed(status=done)
effect_results(trigger=on_done, outcome=ok)
```

The Web run page rendered the terminal run state but omitted the effect result, including after a
reload. The server continued serving the retained `effect_results` frame when queried after the
terminal sequence.

## Root Cause

`useLoopStream` delivers a terminal `status_changed` frame and immediately closes its
`EventSource`. Terminal effects run asynchronously after terminal state is committed, so their
durable `effect_results` frames can be appended after that status frame. During replay, the client
closes at the same terminal frame and never reaches the later retained event.

The server stream is not terminal-status bounded. It keeps polling until the request context is
cancelled. The Loop event reducer and run-story projection already retain and render
`effect_results`; the frame is lost before either consumer sees it.

## Design

### Subscription ownership

The mounted run page owns the Loop event subscription. A terminal run status does not close the
source because it is not an end-of-stream marker. The existing effect cleanup remains the single
owner of listener removal, lifecycle-store closure, and `EventSource.close()`.

The subscription continues to close when any existing ownership boundary changes:

- the hook unmounts;
- the workspace or run identity changes;
- the replay cursor changes;
- the stream is disabled; or
- the injected source factory changes.

The active-subscription identity check continues rejecting frames from replaced subscriptions.

### Event delivery

Named and unnamed frames keep their current parsing path. Every valid frame is delivered to the
latest `onEvent` Effect Event. Lifecycle frames retain their current query invalidation behavior.
A terminal `status_changed` frame and any following `effect_results` frame therefore reach the
consumer in producer order.

No timer, grace period, expected-effect counter, or Loop-definition inspection is introduced.
Those approaches would either race slow effect delivery or couple the transport hook to effect
execution semantics.

### Errors and reconnects

The native `EventSource` remains responsible for reconnect behavior. Connection errors continue
through `onError`; the hook does not close the source in response. Parse failures remain isolated
from consumer failures.

## Alternatives Considered

### Keep the subscription open until its owner releases it — selected

This matches the server's long-lived SSE contract and uses the existing React effect cleanup. It
is the smallest root-cause fix and does not require a public protocol change.

### Add a server end-of-stream marker

An explicit marker after all terminal effects could allow eager client closure, but it would add a
new cross-surface protocol and must account for retries and at-least-once effect delivery. That is
unnecessary for restoring the current UI behavior.

### Close after counting declared terminal effects

The client could inspect the Loop definition and wait for a matching count. This duplicates daemon
semantics, mishandles retries or unavailable definitions, and makes a transport hook dependent on
domain execution policy. It is rejected.

### Close after a grace period

A delay only narrows the race and cannot prove that all effects arrived. It is rejected as a timing
workaround.

## Tests

The canonical hook suite will replace the terminal-close expectation with a regression that:

1. emits a terminal `status_changed` frame;
2. emits a later `effect_results` frame on the same source;
3. verifies both frames reach `onEvent` in order;
4. verifies the source remains open before unmount; and
5. verifies unmount performs the existing cleanup and closes the source once.

The test must fail against the current implementation because the terminal frame closes the fake
source contract before the later frame is accepted. Existing tests continue owning named-event
registration, stale-source rejection, error forwarding, invalidation, and cleanup variants.

## QA

After automated Web tests, typecheck, lint, build, and repository gates pass, isolated QA will use
the daemon authorization fix from issue #403 together with this Web branch. A generic Loop with a
successful terminal tool effect must show the terminal `effect_results` row in run details, both
live and after reload. A failed cross-workspace terminal effect must also remain visible with its
denial outcome. The QA environment must be torn down with `clean: true` and no survivors.

## Non-goals

- Changing Loop effect authorization or delivery semantics.
- Adding or renaming API, SSE, configuration, ToolID, or extension contracts.
- Materializing `loop-effect` as an authored or public agent.
- Changing the visual design of the Loop timeline.
