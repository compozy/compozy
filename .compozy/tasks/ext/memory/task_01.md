# Task Memory: task_01.md

Keep only task-local execution context here. Do not duplicate facts that are obvious from the repository, task file, PRD documents, or git history.

## Objective Snapshot
- Extract shared subprocess lifecycle code from `internal/core/agent` into `internal/core/subprocess`, add `version.ExtensionProtocolVersion`, preserve ACP behavior, add focused subprocess coverage, and finish with clean verification.

## Important Decisions
- Validate the extraction shape against the real ACP SDK before moving code so the refactor stays mechanical and does not invent incompatible seams.
- Rewire ACP only onto the extracted process lifecycle primitives in this task; keep the new generic transport and initialize helpers available for the upcoming extension-manager work without forcing ACP off the SDK transport.

## Learnings
- `internal/core/agent` currently owns process startup, wait, close/kill sequencing, and platform-specific force-termination helpers directly in `client.go` plus `process_{unix,windows}.go`.
- The ACP SDK already implements generic line-delimited JSON-RPC connection handling internally; the new subprocess package will need its own protocol-agnostic transport surface for extension work without breaking ACP behavior.
- `make verify` initially failed only on subprocess-package lint findings (`gocritic` on custom trim logic and `gosec` G204 on the intentional subprocess launch site); replacing the custom trim code and documenting the launcher false positive resolved the gate without changing behavior.

## Files / Surfaces
- `internal/core/agent/client.go`
- `internal/core/agent/client_test.go`
- `internal/core/agent/registry_launch.go`
- `internal/core/agent/session_helpers_test.go`
- `internal/version/version.go`
- `internal/core/subprocess/process.go`
- `internal/core/subprocess/process_unix.go`
- `internal/core/subprocess/process_windows.go`
- `internal/core/subprocess/transport.go`
- `internal/core/subprocess/handshake.go`
- `internal/core/subprocess/transport_test.go`
- `internal/core/subprocess/process_unix_test.go`

## Errors / Corrections
- Corrected the subprocess package lint failures before the final verification run and re-ran the full repository gate successfully.

## Ready for Next Run
- Task implementation is complete: targeted ACP regression tests passed, `internal/core/subprocess` reached 80.4% statement coverage, and `make verify` passed cleanly.
