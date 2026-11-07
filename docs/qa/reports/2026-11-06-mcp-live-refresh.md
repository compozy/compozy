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

## Review corrections and final re-walk

The branch was rebased onto main `59fd1fc7d`. A fresh targeted lab covered the review repairs:
`/Users/pedronauck/dev/qa-labs/compozy-mcp-live-1334-review-20261107-022505-652874-lab/qa-artifacts/qa/bootstrap-manifest.json`.

| Mode | Provider | Existing session | Fresh session / native identity | Result |
| --- | --- | --- | --- | --- |
| Hosted HTTP OAuth | Claude | `sess-54bef95a19f00bc0` | `sess-e3c30ca81ce26114` | Both echo calls succeeded; logout returned ABSENT |
| Hosted HTTP OAuth | Codex | `sess-c6b7a14802f415de` | `sess-f099ff274c5fb77e` | Both echo calls succeeded; logout returned ABSENT |
| Direct stdio | Claude | `sess-114d5761d76f2bea` | `25c2a70e-2239-483f-8b48-1c686fa2528c` | Addition was callable; removal returned ABSENT; native ID retained |
| Direct stdio | Codex | `sess-bac7f845b1e0fd8f` | `01a1b3dd-97dc-7d61-8c0d-1b66d72db64c` | Addition was callable; removal returned ABSENT; native ID retained |

The hosted walk retained the nonresponding HTTP source. All six binds completed in 7.3–19.8 seconds.
Codex loaded native session `01a1b3ec-3f74-75e3-99ea-685d488317e0` successfully after OAuth and logout.
The lab retains `oauth-results.json`, `direct-results.json`, provider responses, daemon logs and the
journey log. Intermediate OAuth attempts are retained separately rather than counted as passes.

Two review walks caught a new-session Codex failure: the registry reported the tool callable,
but its initial provider catalog lacked the callable reference. The registry repeated discovery
while checking projection availability, so an otherwise successful discovery could disappear
within the same initial projection. Availability now uses that request's discovered descriptors
and still checks current authorization; call-time validation remains fresh. The canonical regression
passes after the fix and fails under a Go overlay restoring the previous behavior, without changing
the worktree. The real-provider walk above passed after that repair.

The review repairs also preserve authored and unmarked legacy delegation ceilings, record generated
budget provenance internally, retain explicit native-only narrowing, clear provenance on instance
adoption, use the effective post-hook command, support lazy hosted clients and Pi, and reload direct
stdio sidecars. Long-tail hosted adapters share the turn-boundary mechanism; only Claude and Codex
were exercised with real providers. Direct HTTP/OAuth remain unsupported outside hosted mode.

Publication checks cover default-profile ownership, external profile-file edits/removals, authoritative
Settings rollback including sidecars present at boot, and keeping a rejected user file inactive when
another scope changes. An explicit external edit of the user layer releases that active projection.
No public DTO or SQLite schema changed; optional provenance stays in existing internal policy JSON.

The final focused race selection passed all six affected packages (daemon, session, tools, MCP,
store, API contract). Focused lint returned `0 issues`; the test-shape checker found no new errors
relative to the canonical-suite baselines. The lab's strict evidence auditor passed, and the token
scan found no stub bearer or refresh token in daemon/provider/journey artifacts.

Known Settings boundary from independent review: an explicitly written workspace/profile definition
may remain published if a different runtime subsystem rejects the broader apply. The apply record
remains failed and scope isolation remains intact. This work preserves that partial-apply contract;
it does not add a multi-resource transaction. User-layer rollback uses the full active snapshot and
stays pinned through unrelated scope edits or transient file observation errors.

The final real-daemon integration passed again in 42.903 seconds (`term-2798717b9c15`).
`make codegen-check` passed with no generated drift (`term-073b185ba715`). The review lab's
canonical teardown reported `TEARDOWN_CLEAN=true` and `TEARDOWN_ALL_CLEAN=true`
(`term-83526ef8767b`); copied credentials and owned processes were removed.

MCP publication reads registered workspace identities directly, rather than resolving inherited
workspace configuration before reading each scoped declaration. This keeps malformed user config
from blocking healthy workspace/profile publication; the declaring-scope suite owns that invariant.

The next Greptile round found that one inaccessible workspace path could stop observation of all
other layers. The declaring-scope suite now replaces one temporary workspace directory with an
unreadable path, changes a healthy user declaration, verifies its update and the failed scope's
retained records, then restores the path and verifies recovery. That regression failed before
the production repair. File observations now continue independently; scoped declaration loading
retains unreadable records. The CI preflight also caught one retired product-name occurrence in
the official skill; it was corrected and the focused product-language check passed.

After this repair, the focused daemon publication/rollback selection passed with race (11.852
seconds), focused daemon lint returned zero issues, and the real-daemon OAuth integration passed
again (43.114 seconds). The CI Settings failure was an old restart-required expectation for MCP
repair/add: the owning canonical test now requires the approved live lifecycle, successful apply
and no restart while retaining its auth-failure and persisted-definition assertions. That focused
Settings race selection passed (4.108 seconds).

A later CodeRabbit round identified two additional boundaries. Cold Settings services now
initialize the active snapshot before section/collection writes, so the first failed live mutation
retains the pre-write config. Canonical Settings regressions reproduce section, upsert and delete
failures without warming ActiveConfig first. Extension MCP grants now compare the logical config
name while retaining the allocated runtime name for tool IDs and routing; credential ownership is unchanged. The policy context
resolves that logical identity by the descriptor's exact resource ID from the current catalog;
the owning policy regression rejects runtime aliases and unrelated logical grants. Both failures
were reproduced before their production fixes. The focused Settings/tools/daemon race selection
passed after repair (10.478, 1.931 and 20.091 seconds respectively).

The final policy implementation resolves logical names in the per-request policy context by exact
resource ID, preserving the public source/descriptor shape. Its daemon policy regression replaces
the catalog name while holding the runtime allocation stable and verifies that the old logical
grant stops authorizing it. Focused race tests passed Settings/tools/daemon (11.170, 1.587 and
10.089 seconds); focused lint returned zero issues. Codegen-check passed without drift, and the
preceding real-daemon OAuth walk passed in 46.148 seconds.

The real-daemon OAuth integration passed again on the final resource-name policy implementation
(39.948 seconds). The separate triage-cleanup CI assertion did not reproduce in three focused
race runs; its production and test files match main and were left unchanged while CI evidence
was investigated. This is not reported as a passed CI check.
