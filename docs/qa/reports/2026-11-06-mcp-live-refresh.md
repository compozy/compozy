# MCP live refresh — issue #1334

PASS for the targeted backend and real-provider walks. Broader MCP interoperability scenarios retain their previous status.

## Environment

Isolated worktree `fix/1334-mcp-live-session-refresh`, based on `2f94d9968`. A worktree-built CLI ran a real daemon with a unique home, socket and HTTP port. A local remote-HTTP MCP endpoint required OAuth through a local issuer. Real Claude Opus 5.5 and Codex GPT-6.1-Sol sessions used their normal ACP adapters. A second HTTP endpoint never answered initialization; all six hosted binds completed in 12.5–19.6 seconds within the 30-second launch window. External source policy was explicitly enabled; no branas or AdvOS configuration or credentials were changed.

Manifest: `/private/tmp/compozy-qa-1334/compozy-mcp-live-1334-final-20261107-005853-730742-lab/qa-artifacts/qa/bootstrap-manifest.json`.
Evidence root: the manifest's `qa-artifacts` directory. `oauth-results.json` records the calls; `daemon-oauth.log` records native loads; `qa/qa-audit-report.json` passed the strict evidence audit.

## Observed behavior

| Provider | Existing session | Session created after OAuth | After logout |
| --- | --- | --- | --- |
| Claude | `sess-58a6b3b338ac5764` called echo with `LIVE_OAUTH_OK` | `sess-adaa79a2e27f4490` called echo with `NEW_OAUTH_OK` | Existing session reported unavailable; no echo call |
| Codex | `sess-9eb7c37a9cc764f3` called echo with `LIVE_OAUTH_OK` | `sess-88cc8af4a3be2bf2` called echo with `NEW_OAUTH_OK` | Existing session reported unavailable; no echo call |

Claude updated its stable MCP connection. Codex loaded native session `01a1b3a6-aac6-7481-ad6d-ebe6e35a0793` successfully at 01:37:24Z after authorization and 01:39:23Z after logout. Both loads retained the original native ID; the final walk had no active-writer error or transcript-replay fallback. Logical session IDs stayed unchanged.

The focused `TestDaemonHostedMCPLiveAuthRefresh` integration ran a real daemon with the canonical OAuth HTTP fixture. One subscribed MCP client connected before the server was declared, then listed and called it after OAuth. A second session created after Ready also listed it. Disabling source policy withdrew tools from both clients and rejected calls; re-enabling restored them. Logout withdrew it, reauthorization restored it, and deleting the declaration removed it from both clients and rejected calls.

Earlier failed walks were repaired and repeated: the test client initially used an unsubscribed cached list; its cleanup canceled the command before graceful close; Claude's first list used a native-only bootstrap; Codex initially attempted native load before releasing the previous writer. Later walks caught a partial bootstrap incorrectly cached as complete, a subscription race before the first full list, and an initial snapshot reused across a reload. The registry now separates bootstrap completeness, and session-hosted bindings deliver a fresh full initial catalog. The final walk above passed after fixing those causes.

## Focused verification

- `go test -tags=integration ./internal/daemon -run '^TestDaemonHostedMCPLiveAuthRefresh$' -count=2 -v -artifacts -outputdir /private/tmp/1334-integration-artifacts`: both runs passed (92.610 seconds total).
- Scoped `go test -race` covered publication, profile ownership, root identity refresh, delegated narrowing, native writer release, projection readiness, transient-error preservation, retired-bind handling and effective tool policy.
- Four Claude Opus 5.5 high review rounds examined the full diff. Findings were fixed with canonical regressions; the explicit-tool/server-grant question was answered by preserving the server ceiling and documenting the prior bypass correction.
- `make codegen` and `make codegen-check`: passed after installing the repository's pinned patched dependencies. The generated lifecycle matrix now classifies MCP declaration mutations as live.
- `audit-qa-evidence.py --qa-output-path .../qa-artifacts --strict --explain`: passed.
- Canonical `teardown-qa-env.py --manifest .../bootstrap-manifest.json`: `TEARDOWN_CLEAN=true`, `TEARDOWN_ALL_CLEAN=true`; copied provider credentials and owned runtime processes were removed.

CI owns repository-wide gates and E2E. No local full gate, full suite or E2E was run. Real vendor consent, the reporter's production services and unrelated historical MCP journeys remain outside this targeted evidence.
