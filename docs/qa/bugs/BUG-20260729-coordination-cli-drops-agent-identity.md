# BUG-20260729-coordination-cli-drops-agent-identity: Agent CLI coordination reads bypass workspace access policy

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** High · **Priority:** P0
- **Persona Affected:** Ada
- **Journey Step:** J-cross-workspace-access, coordination seam
- **Scenarios:** ET-workspace-access-mode-matrix
- **Found:** 2026-07-29 · **Report:** docs/qa/reports/2026-07-29-cross-workspace-access.md
- **Origin:** CH-cross-workspace-mode-seams live QA

## Summary

`compozy network coordination status --workspace <foreign>` drops the current session identity before its UDS request. A `deny-all` agent therefore reaches the route as the local human operator and reads the foreign workspace instead of receiving the daemon-origin permission-mode denial and exit code 77.

## Reproduction

- **Charter:** CH-cross-workspace-mode-seams · **Tour:** Feature Tour
- **Environment:** fresh isolated `northstar-pay-20260729-124649-419333` lab; daemon `412ab876`; source `ws_d8316abd12c3ac51`; target `ws_fa3d006864b805e1`.

1. Start session `sess-b1f207ee4fba1770` for global agent `qa-deny-all` in the source workspace.
2. Export `COMPOZY_SESSION_ID=sess-b1f207ee4fba1770` and `COMPOZY_AGENT=qa-deny-all`.
3. Run `compozy network coordination status --workspace ws_fa3d006864b805e1 -o json` from the source workspace.
4. Repeat the same GET over HTTP and UDS with `X-Compozy-Session-ID` and `X-Compozy-Agent` headers.

**Expected:** All three surfaces return the same daemon-owned denial; CLI exits 77 with the permission-mode hint.

**Actual:** HTTP and UDS return 403 with the correct hint. CLI exits 0 and returns the target coordination record with `resolution_source: cross_workspace_attempt`.

## Evidence

- Live CLI task control: the same identity correctly exits 77 on a foreign `task create --as-agent` request.
- Live HTTP/UDS controls: both coordination requests return byte-equivalent 403 diagnostic payloads.
- Source trace: `internal/cli/client_network_coordination.go` calls `doJSON` while agent-aware client surfaces call `doAgentJSON` with explicit credentials.

## Fix

- **Root cause:** Confirmed: the coordination client contract has no credentials parameter, so status, compare-and-set, and invitation calls cannot forward session identity headers.
- **Correction:** Coordination status, compare-and-set, and invitation calls now take the validated
  session credentials and use the shared agent-aware UDS transport. Command wiring reads the same
  environment identity as the other agent CLI surfaces.
- **Fix commit:** `4ef8e8c`
- **Regression test:** `TestUnixSocketClientNetworkMethods/Should_forward_agent_identity_on_coordination_requests` owns the CLI transport invariant.

## Verification

- The canonical transport regression failed before the correction because the coordination request
  carried no agent headers and passes afterward.
- `go test -race ./internal/cli/...` passed 1,329 tests across two packages.
- The rebuilt isolated daemon replayed `deny-all` and `approve-reads` as exit 77 with the exact
  daemon hint, while `approve-all` exited 0. HTTP and UDS remained byte-equivalent 403 controls for
  the denying identities.
- `make lint`, `make test-e2e-runtime`, and `make test-e2e-web` passed after the source freeze.
