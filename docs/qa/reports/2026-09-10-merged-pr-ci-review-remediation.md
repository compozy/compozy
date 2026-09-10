# Merged PR CI and review remediation

The operator authorized squash merging these five PRs before all checks passed,
then fixing remaining CI and review findings directly on `main`.

| PR | Squash commit | Result |
| --- | --- | --- |
| #596 — Layout settings recovery | `cd474b070d5ebecd21f7f0634f954ecc83f6ff25` | Merged |
| #597 — Terminal close confirmation | `4b28139736af27634d6f46c73b4655513624998b` | Merged |
| #599 — Connected-session CPU | `905c0091a5240f369647c70a53ad369eccf6ec66` | Merged |
| #600 — Session summaries | `d17da5d9faec1e3ce8ae1b5bac79a97c55ad2153` | Merged |
| #601 — Goal lifecycle | `be71ed84e6cd183a75979587616f9a21e3a95ff9` | Merged |

## Review disposition

All issue comments, review bodies, inline comments, and review threads were read
through paginated GitHub APIs. The captured inventory has 28 threads: six for
#596, four for #597, seven for #599, four for #600, and seven for #601. Three
remained unresolved when remediation began.

- **#597, discussion r3982537005:** valid. Terminal creation now captures the
  initiating workspace, profile, query scope, coordinator, and shell binding in
  mutation variables. Completion invalidates the original reads. The live
  coordinator is invoked only while the original binding remains current, so
  switching away and back cannot revive an obsolete navigation. The created
  terminal remains discoverable in its owner's catalog. Returning to the
  initiating tab offers **Open terminal** for that completed process; it does
  not remain in a loading state or create another process. The action appears
  only in the initiating workspace and profile and requires an explicit click.
- **#600, discussion r3982411495:** partially fixed in the merged head. The
  component imports already used the session barrel, but icon/name helpers
  still bypassed it. Those imports now use the same public barrel.
- **#600, discussion r3982367426:** existing coverage owns this domain behavior.
  `session-thread.test.tsx` and the `LongSummaries` / `LongSummariesParallel`
  production-composition stories already exercise full details, keyboard
  disclosure, and focus restoration. The thread suite now also verifies pointer
  activation and exact tool/activity clipboard contents. A duplicate component
  harness is unnecessary under the current test-placement rules.
- **#596 palette-suite nitpick:** the merged ownership header and existing
  root/surface suite already address it. There is no separate surface suite to
  move the test into. The palette fix is grounded in the prior Herdr E2E failure.
- **Documentation warnings:** #596/#599 were reviewed against their later
  remediation commits and existing function comments. Missing lifecycle and
  presentation comments in #600/#601 now state their relevant contracts. The
  excluded QA reports, scenarios, official references, and site documentation
  were inspected; their original verification limits remain explicit.

## CI findings

- **#599 Go lint**, job `103008223478`: `gocritic/dupArg` rejected
  `snapshot.Equal(snapshot)`. The existing `TestFromInfo` invariant remains:
  failed fallback reads cannot authorize reuse. It now compares two independent
  captures of the unavailable file with the same retained metadata.
- **#600 Web E2E**, job `103006561182`: the dashboard journey found two buttons
  named `Home`. The new title popup inherited the title as its action name.
  Topbar now names that control `Show full title: Home` while retaining `Home`
  as the heading's name and preserving the route focus target. The dashboard
  assertion is unchanged. The existing Topbar test and long-title story follow
  the explicit action name.
- **#600 Teams race shard**, job `103006561149`: the provider readiness marker
  did not arrive within the existing deadline. Ten repetitions of the exact
  test and three complete Teams suite runs passed locally with `-race`. No
  timeout, readiness assertion, or production behavior was weakened. Final CI
  must validate the Linux shard again; the original timeout is not claimed to
  have a proven root cause.
- **Post-merge Web gate:** the visual-state suite still expected the free-form
  name `Read /tmp/a.ts` to resolve as canonical `Read`. That assertion predates
  the PR's explicit rule that identity is never inferred from a prose prefix,
  reinforced by accepted review `r3982359901` and the current tool-label suite.
  The owning visual-state suite now checks both canonical `Read` as `read`
  and the descriptive name as `other`. No production prefix heuristic was
  reintroduced to satisfy the stale assertion.

## Verification record

Local logs and the complete GitHub inventory are retained under the ignored
`.cache/merge-prs-596-601/` directory. Verification distinguishes the existing
real daemon/browser journeys from deterministic Query/clipboard I/O tests.

- The focused Topbar, session-thread, and terminal-controller suites passed:
  172 tests. The controller's 22 tests passed again after adding recovery across
  host remounts. Root Turborepo Web typecheck passed after the final change.
- The production Web build passed. It retains the existing mixed-import,
  chunk-size, and plugin-timing Vite notices; those notices are not represented
  as a clean warning-free build.
- React Doctor found no issues in the changed production code.
- `make gate` passed: scoped Go lint/race tests, UI lint/typecheck and 762 tests,
  Web lint/typecheck and 7,187 tests. The final log is `gate-final.log`; the
  earlier `gate.log` retains the stale-expectation failure and its diagnosis.
  Both lint lanes reported zero warnings and errors. `make gate-status`
  records the resulting lane evidence.
- Pre-commit tasks passed with `--no-stash --no-hide-partially-staged`, and
  commitlint passed. Automatic hooks are disabled only for the final commit
  because the default pre-commit command would run the prohibited Git stash.
- The post-push CI result is retained as `final-ci.json` in the same evidence
  directory. A manually dispatched full CI run covers backend, Web, and
  Desktop; the earlier merged-head run was canceled by another main update,
  and the subsequent docs-only run does not replace that full verification.

### Isolated browser journey

PASS for the changed terminal creation journey and the Home title control.
The current production bundle ran against the real daemon at
`http://127.0.0.1:59016`. The browser held a real successful terminal-creation
response, switched to another workspace, then delivered that response. The
destination window-manager snapshot remained exactly equal and its terminal
catalog remained empty. Returning to the owner displayed **Terminal ready**;
clicking **Open terminal** attached to the original process without increasing
the terminal count. A command typed in that recovered terminal produced
`scope-recovery-ready` in the daemon's screen read. Home retained one Dock
button named `Home`, a heading named `Home`, and a separate **Show full title:
Home** action with full text and focus restoration.

The first re-walk exposed the abandoned launcher's loading state, which was
fixed before the successful final re-walk. Completed creations are recovered
from the mutation cache, so reconstruction of the controller does not erase
the recovery action. Scratch harness fixes waited for the workspace menu and
terminal connection instead of reading transitional UI state.

Lab evidence root:
`/Users/pedronauck/dev/qa-labs/compozy-merged-pr-terminal-scope-20260910-192704-920206-lab/qa-artifacts/qa/`.
The root contains `bootstrap-manifest.json`, `journey-log.jsonl`,
`runtime-result.json`, `runtime-trace.zip`, `home-title.png`,
`destination-after-create.png`, `owner-ready.png`, and `owner-returned.png`.
The lab uses an isolated home and socket. No provider-backed agent session
was required for this targeted terminal/window journey. Workspace switching
was walked live; profile switching and leave/return-before-completion are
covered by the real Query/window-runtime controller suite.
The manifest teardown completed with `teardown.json` reporting `clean: true`.
The strict evidence audit passed with zero blockers and warnings; its result is
`qa-audit-report.json` in the same lab root.

The terminal controller suite owns creation-scope regressions with real Query
mutation observers, routing coordinator, and window runtime. Only terminal
creation and window-command transport are replaced at their I/O boundaries.
Cases cover unchanged binding, workspace switch, profile switch, unbinding,
and leaving then returning to the original scope, including the completed
terminal's availability for explicit recovery only in its initiating scope.

## Cross-surface impact

- **Native tools:** terminal creation/close, window operations, and Goal tools
  retain their IDs, schemas, and policy gates; no transport contract changed.
- **Extensibility/hooks/config:** no hooks, SDK, registry, settings, or bridge
  runtime changes. The filesnap change is confined to its existing test.
- **Workspace isolation:** terminal creation and cache reconciliation use the
  captured workspace/profile. A shell binding change prevents late navigation
  into the current desktop. No persisted state or schema migration is involved.
- **Official skill:** the merged Terminal and Loop references retain the same
  operator/API contracts and do not need another command or capability entry.
- **Web/Docs:** the terminal controller, session barrel, and shared title
  disclosure are affected. Existing terminal/transcript QA scenarios and this
  report record the changed behavior. Go changes outside filesnap tests are
  function documentation only.
