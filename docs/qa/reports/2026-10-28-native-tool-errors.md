# Native tool error parity — issue #1012

- Scope: native error codes, sanitized causes, and CLI/native gateway parity.
- Cadence: targeted.
- Build: `fix/issue-1012-native-tool-errors`, based on `681a2ea30`, with the implementation in this PR.
- Environment: isolated daemon built from this checkout; separate home, socket, port, and disposable repositories. No operator daemon mutations.
- Verdict: PASS for the targeted journeys below. CI and external reviews are separate delivery gates.

## Charter and results

Ada, an operator using CLI and native tools, needs to distinguish a state refusal from an outage before retrying. The error-handling tour used public CLI commands and `tool invoke` against the real native registry. No database edits or backend stubs were used in the lab. A real Codex session delegated the README summary to produce the child session.

| Journey | CLI/native observation | Independent read and result |
| --- | --- | --- |
| Archive a subagent child | Both return `subagent_archive_follows_parent` and explain parent-owned archival | Child status remains available, not archived; PASS |
| Archive a detached session | Both return `session_archive_requires_stopped` | Status confirms `detached` with lifecycle `active`; PASS |
| Create a duplicate worktree | Both return `worktree_name_taken`, naming the collision | Original worktree ID retained; PASS |
| Remove a session-bound worktree | Both return `worktree_session_active`; native message explains the active session | Inspect retains the ready checkout and binding; PASS |
| Workspace prune failure | CLI names prune and active sessions; native retains `tool_backend_failed` / `backend_unhealthy` and the same cause | A separate registered lab root was temporarily renamed, then restored; no new worktree was created; PASS |
| Recovery after stopping | Native archive succeeds once the session is stopped | CLI archived catalog confirms the persisted archive; PASS |
| Discovery/schema rejection | Live descriptors are readable; an incorrect remove field returns `tool_invalid_input` / `schema_invalid` | Correct descriptor field `ref` used in the successful re-walk; PASS |

## Findings and re-walk

The initial real child returned the store-layer archival sentinel, unlike the session-layer sentinel used in the first regression. The shared domain payload/status mapping now recognizes both. The test table covers both identities.

Restarting the lab daemon stopped the original sessions, so the first post-restart attempt exercised successful archival/removal instead of state refusals. Fresh active sessions restored the required preconditions. Those setup attempts remain in the evidence log; only the subsequent `rewalk-*` receipts establish detached, binding, and prune refusal results. Child and duplicate receipts are `final-child-*` and `final-duplicate-*`. Recovery receipts are `recovery-*`.

## Automated evidence

- `go test ./internal/api/core ./internal/daemon ./internal/tools -run 'Test.*(Tool|Worktree|Archive)' -count=1`: PASS. This includes existing specialized Loop reasons, partial-result envelopes, access boundaries, process failures, and the new regressions.
- `COMPOZY_GO_LINT_SCOPES='./internal/api/core ./internal/daemon ./internal/tools' make go-lint`: PASS, zero issues for default and integration tags.
- `make codegen` with pinned Bun 1.4.0 and frozen dependencies: PASS, no generated diff. An initial run without local dependencies produced incidental generator drift; installing the existing lockfile and regenerating removed it.
- Test-shape heuristics pass for the changed native/tool suites. The worktree file reports a pre-existing direct assertion in `TestCreateSessionWorktreeBinding`; the added registered-code table uses parallel `Should` subtests.
- The core suite checks every registered worktree domain code, status, cause, and wrapped identity. Backend redaction coverage includes credentials and a quoted private path containing spaces.
- No `make gate`, full race lane, or full local E2E was run, as instructed; CI owns delivery gates.

## Evidence and limits

Local artifacts live under the bootstrap-generated `qa-artifacts/qa/` directory: manifest, process identity, journey log, command stdout/stderr, provider-created parent/child receipts, and final audit/teardown receipts. Private absolute paths are omitted here. The strict audit passed with no blockers; teardown reported `TEARDOWN_CLEAN=true` and `TEARDOWN_ALL_CLEAN=true`.

The live prune cause contained no credentials or host path. Sensitive-cause redaction is verified by the owning automated suite, not by seeding real credentials into the daemon. This targeted run does not promote the broader scenario catalogs to a full release PASS. CI and CodeRabbit/Greptile review outcomes are recorded on the PR. No merge is authorized.

## Review remediation

Greptile identified unquoted paths containing spaces; CodeRabbit requested case-specific message assertions. The API suite now checks quoted Unix, unquoted Unix, and unquoted Windows paths, including absence of every private fragment. Both unquoted cases failed before the production fix and pass afterward. The native adapter suite requires the colliding name, bound session ID, or permission-denied cause for its respective case.

A rebuilt lab daemon was exercised against an existing directory with spaces through CLI and native worktree creation. CLI returned its operator path; the native result was exactly `worktree_path_exists: [path]`. The refreshed targeted suites passed; the laboratory was torn down again with a clean receipt. Review changes retain the original wire shape and domain behavior.
