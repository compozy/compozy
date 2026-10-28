# Hosted MCP reconnect after an Agy model switch

Date: 2026-10-19. Verdict: **PASS — scoped reconnect regression**.

## Incident and repair

Session `sess-dcba53bda9072a50` successfully bound hosted MCP, then began returning
`hosted_mcp_nonce_invalid` after switching Agy models. The daemon recorded a successful bind at
12:25:44Z and rejected replacement proxy PIDs from 12:42:13Z onward. The installed Agy 1.4.1
implementation rebuilds its agent and MCP proxies at the next turn after a model change, retaining
the original launch arguments. Compozy permanently consumed that launch nonce at the first bind.

The authenticated current proxy's release now rearms one reconnect window using the existing
configured nonce TTL. Each proxy gets a distinct bind; concurrent binds, unauthorized releases,
expired windows, and replaced/stopped-launch credentials remain rejected. Stale releases cannot
reopen or extend a window. Workspace/profile scope and active-run identity remain unchanged.

## Isolated account-backed walk

- Fresh bootstrap manifest:
  `/tmp/opencode/qa/compozy-hosted-mcp-reconnect-20261019-125117-009307-lab/qa-artifacts/qa/bootstrap-manifest.json`.
- Separate daemon home `/tmp/compozyqa-b562fae0f00d/runtime`, port `52335`, built binary
  `/tmp/opencode/compozy-hosted-mcp-reconnect`; native provider login remained in the operator home,
  without copying credentials or changing authentication.
- Real Agy 1.4.1, one logical session `sess-39d70d2e80ae0b70`, one ACP session
  `22342ffe-72c8-4abb-aefc-a32c7f10e898`.
- Flash → Argon → Flash, Low effort: all three prompts invoked `compozy__workspace_info`
  successfully and returned `MCP_RECONNECT_OK`. Three distinct proxy PIDs bound successfully;
  zero hosted bind failures. HTTP session reads confirmed `live_configuration`, the selected model,
  and the unchanged ACP session ID. CLI history and tool-completion logs confirmed all three reads.
- Evidence directory:
  `/tmp/opencode/qa/compozy-hosted-mcp-reconnect-20261019-125117-009307-lab/qa-artifacts/qa/`.
  Receipts: `prompt-flash-initial.json`, `prompt-argon-switch.json`, `prompt-flash-return.json`,
  `session-argon-switch.json`, `session-flash-return.json`, `hosted-bind-outcomes.json`,
  `tool-completions.json`, and `history.json`.

## Automated verification

- Full race-enabled `internal/mcp` and `internal/api/udsapi` suites pass. Coverage: 79.4% and 84.4%.
- `TestHostedServiceBindNonceLifecycle` reproduces the rejected replacement before the repair and
  covers authenticated reconnect, inherited scope/run, unauthorized release, concurrent replay,
  stale release, bounded expiry, replaced launches and session revocation.
- Owning Go lint lane, including integration tags, and test-shape checks pass. Windows cross-build
  of `internal/mcp` passes; this is not Windows runtime evidence.
- Final verify: `/tmp/opencode/qa/compozy-hosted-mcp-reconnect-20261019-125117-009307-lab/qa-artifacts/qa/verify-tests.log`.
- Final verify: `/tmp/opencode/qa/compozy-hosted-mcp-reconnect-20261019-125117-009307-lab/qa-artifacts/qa/verify-lint.log`.
- Strict evidence audit passes (`qa-audit-report.json`). Targeted lab teardown reports
  `clean: true` in `teardown.json`; no owned lab processes remain. The helper escalated the
  isolated daemon after graceful stop did not finish; the operator daemon was not targeted.

## Limits and operator decision

The original daemon and session are not modified: the operator explicitly selected code-only
delivery. Applying the fix still requires a rebuilt daemon and a fresh runtime binding.
No commit, push, full `make gate`, browser walk, Windows/macOS runtime walk, abrupt proxy-kill
recovery, or broader `ET-048` scenario is claimed. MCP coverage was already below the 80% project
target: a read-only HEAD overlay reports 79.2%, versus 79.4% with this change. Its first baseline
run hit the existing concurrent-projection-cache test flake; the unchanged baseline passed on retry.
No unrelated coverage or cache changes were made.
