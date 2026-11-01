# Dependency patches

`@hey-api/openapi-ts@0.99.0` ships an SSE template whose abort handler discards the
Promise returned by `reader.cancel()`. If aborting fetch has already errored the
reader, cancellation rejects and can terminate the Bun host through an unhandled
rejection. [This generated-client reproduction](https://github.com/anomalyco/opencode/issues/47963)
matches the real TypeScript SDK journey on Bun 1.4.0.

The Bun patch consumes that cleanup Promise in the template used by both generated
SDK jobs. Generated files are still produced and checked by `make codegen` and
`make codegen-check`. Transport and parsing errors retain their existing paths.

Remove this patch and its `patchedDependencies` entry when the pinned generator's
SSE template handles cancellation rejections itself. Keep the existing client
abort regression and rerun `TestDaemonE2EAppSDK` with the public built package to
verify that stopping an event reader leaves the host alive without replaying work.

`@assistant-ui/core@0.3.31` starts `ToolInvocationTracker`'s stream pipeline in its
constructor and never disposes that pipeline. React can abandon the constructor's
render before effects commit. Chromium's pending stream reader then retains the
tracker, its adapter callbacks, and the old React tree. The same pipeline survives
ordinary session unmounts. A production-browser heap trace with 64 active sessions
identified this retaining path through `ReadableStreamDefaultReader` and `_getTools`.

The Bun patch changes the published runtime at that ownership boundary: create the
pipeline on the first live tool entry, abort it on the existing permanent thread
disposal signal, and dispose it when the adapter disables tool invocations. Normal
reset/cancellation remains reusable, including StrictMode replay. Late results from
a permanently disposed tracker cannot update its former owner; genuine pipeline
errors still use the existing error/restart path.

Since 0.3.31 the tracker is retained per message repository and can move to a new
host. A host that was already replaced no longer holds the tracker, so its disposal
is a no-op. A disposing owner also drops the retained entry, so a later host builds
a live tracker instead of adopting a disposed one.

Remove this patch and its `patchedDependencies` entry when the pinned core provides
both lazy stream creation and permanent pipeline disposal. Keep the lifecycle
regression in `session-chat-runtime-provider.test.tsx`, which exercises an abandoned
Suspense render, StrictMode remounts, and real backend-tool stream processing. Repeat
the browser navigation/heap comparison documented in
`docs/qa/reports/2026-10-26-session-responsiveness.md` before removing it. No upstream
issue was created as part of this change.
