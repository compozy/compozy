# Task Memory: task_14.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot

- Ship the public Go SDK under `sdk/extension` with initialize handshake handling, typed hook registration, Host API client, and author-side testing utilities.
- Provide a public type-sharing seam for hook/runtime payloads without importing any `internal/` packages from the SDK.

## Important Decisions

- Keep the public authoring surface under `sdk/extension` and duplicate runtime-facing payload/Host API structs there, guarded by reflection drift tests in `sdk/extension/compat_test.go`, instead of importing `internal/` packages or adding another public type package.
- Follow `_protocol.md` as normative for the initialize direction: the SDK serves a host-originated `initialize` request even though parts of `task_14.md` describe a client-originated handshake.
- Treat automatic `host.events.subscribe` registration as a real contract failure: filtered `OnEvent` handlers now fail the extension session if the host cannot accept the subscription request.

## Learnings

- The runtime handshake/session wiring in `internal/core/extension/manager_spawn.go` already matched a host-originated initialize flow, so the SDK could mirror the live runtime contract without modifying task 08.
- A dedicated SDK-backed subprocess fixture under `internal/core/extension/testdata/sdk_extension` is enough to prove real stdio compatibility with the runtime manager while keeping the existing `mock_extension` fixture intact for lower-level lifecycle tests.
- `sdk/extension` now clears the task coverage gate with `82.8%` statement coverage, and `sdk/extension/testing` reports `82.2%`.

## Files / Surfaces

- `sdk/extension/{doc.go,types.go,hooks.go,transport.go,host_api.go,extension.go,handlers.go,compat_test.go,extension_test.go,host_api_test.go,internal_test.go,smoke_test.go}`
- `sdk/extension/testing/{mock_transport.go,harness.go,harness_test.go,mock_transport_test.go}`
- `internal/core/extension/{manager_test.go,sdk_manager_integration_test.go}`
- `internal/core/extension/testdata/sdk_extension/main.go`

## Errors / Corrections

- `go test -cover` exposed an out-of-order Host API response bug in `sdk/extension/host_api_test.go`; the test now correlates responses by method name instead of assuming receive order.
- `make verify` surfaced unchecked transport writes and silent auto-subscribe errors; `sdk/extension` and `sdk/extension/testing` now propagate those failures explicitly so lint and runtime behavior agree.

## Ready for Next Run

- Task complete. Follow-on work should build task 15 (TypeScript SDK and author docs) against the public Go SDK shape and the initialize direction captured in `_protocol.md`.
