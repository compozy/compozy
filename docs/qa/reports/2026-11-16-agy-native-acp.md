# QA Run Report — 2026-11-16 — Antigravity native CLI ACP

- **Scope:** `feat/agy-native-acp`; replace the inherited standalone server with `agy acp`, preserve explicit intent/history, and retain native auth and host authority.
- **Cadence tier:** targeted.
- **Builds:** baseline `1141d8640`, native launch `031a13703`, catalog/daemon `3edb8010d`, and integrated implementation `8d501bba2`. The last live binary was `v0.7.2+agy-native-final.8d501bba2`, SHA256 `0ba12e9b10d948885b5774354c44933026a78149e96ae8fc76ce0c3fcde64046`, as recorded by the build tool result before restart.
- **Final local code review:** `9f12f850e4661b161b17ccd49663c966366f91e9` distinguishes recovery reasons and adds owning-suite coverage. Live Google/browser journeys were not repeated for this diagnostic/test correction.
- **Environment:** Linux x86_64; isolated daemon at `http://127.0.0.1:41805`; operator HOME retained for native CLI authentication. Provider usage collection is disabled.
- **Started:** 2026-11-16T21:40:32Z · **Status:** blocked-verify after the 2026-11-17 server/container restart.
- **Original lab manifest:** `/tmp/opencode/compozy-agy-native-acp-20261116-20261116-213657-800319-lab/qa-artifacts/qa/bootstrap-manifest.json` (lost with the temporary lab).
- **Owning contracts:** workspace-root `.compozy/tasks/agy-native-acp/_tests.md` and `analysis/change-impact.md`.
- **Evidence limitation:** the restart removed the lab, runtime home, screenshots, scripts, journey log, and raw receipts before final sanitization, strict audit, and manifest teardown. The observations below retain the actual successful and failed walks reported in this session; their filenames are historical receipt identifiers, not currently readable artifacts. They do not constitute an audited PASS. No successful continuity prompt was repeated or synthetic receipt substituted.
- **Post-restart engineering evidence:** workspace-root `.compozy/tasks/agy-native-acp/evidence/` holds focused verification and lifecycle results on persistent storage. These are engineering checks, not replacements for the lost provider/browser evidence.

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
| --- | --- | --- | --- |
| Ada | Existing native CLI operator | Linux desktop / wifi-fast / en-US | CH-agy-native-cli-operation, CH-agy-native-session-continuity |
| Sol | Keyboard-reliant operator | Desktop / wifi-fast / en-US | CH-agy-native-runtime-picker |

## Flows in Scope

- [J-15](../journeys/J-15-operate-session-via-cli-api.md): operate a durable session through CLI/API, including provider readiness, permissions, and recovery.
- [J-17](../journeys/J-17-session-create-unified-selector.md): select an advertised runtime and inspect its effective controls.
- Adjacent canary: provider Settings native-auth ownership, without credential entry.

## Session Matrix & Results

Every row was created `Pending` before any implementation-build journey.

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
| --- | --- | --- | --- | --- | --- | --- | --- |
| 1 | CH-agy-native-cli-operation | J-15 / RT-agy-acp-provider: installed CLI, shared login, Hydrogen/xhigh, persisted reply, unsupported max | Ada | Feature Tour | Blocked (needs human verify) | Walk succeeded; raw receipt archive lost on restart | `031a13703`, `8d501bba2` |
| 2 | CH-agy-native-cli-operation | J-15 / RT-provider-full-access-mode: allowed writes, constrained-start refusal, native read tool, cancellation and explicit process cleanup | Ada | Feature Tour | Blocked (needs human verify) | Repaired walk succeeded; raw receipt archive lost on restart | `8d501bba2` |
| 3 | CH-agy-native-session-continuity | J-15 / RT-agy-acp-provider: actual old-to-new continuity and subsequent native restore | Ada | Interrupt Tour | Blocked (needs human verify) | Refusal, confirmation, replay, and native load succeeded; archive lost on restart | `8d501bba2` |
| 4 | CH-agy-native-cli-operation | J-15 / RT-agy-acp-provider: clean-home non-interactivity and separately consented login | Ada | Feature Tour | Blocked (needs human verify) | Passive clean-home probe passed; new consent/cancellation not initiated | |
| 5 | CH-agy-native-runtime-picker | J-17 / ET-web-runtime-selector-minimal-slider; MS-web-provider-auth-gate: Settings, keyboard selection, narrow pane, submitted reply | Sol | Feature Tour | Blocked (needs human verify) | Walk succeeded; captures lost on restart; unknown/stale-source legs tracked separately | |
| 6 | CH-agy-native-runtime-picker | J-17 / browser unknown/stale model-source states | Sol | Feature Tour | Blocked (needs human verify) | Existing source coverage updated; CI E2E not run locally | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`.

## Session Debriefs

### Prior-runtime continuity fixture

- Built baseline `1141d864060678bfec1b75990973fba1b5a87bf1` as `v0.7.2+agy-prior.1141d8640` and started it only in this lab.
- The standalone server 1.4.5 passed the public remote readiness probe as `authenticated`, without starting consent.
- Created `agy-continuity` with no authored raw ACP options, then logical session `sess-46622438439d0da1` in workspace `ws_de6d187861831409`.
- Its first public prompt selected `agy / gemini-4-hydrogen / xhigh` without raw options and returned `Stored AGY-CONTINUITY-7429.`. Public session/history/events reads succeeded.
- The acknowledged old controls are `thinking_level=xhigh` and `mode=yolo`; native ID is `5925404f-7ac9-490d-9a95-335ebd4a8e00`. Both requested/effective projections contain the derived mode; those projections alone do not prove authored intent.
- Evidence under the manifest's `QA_OUTPUT_PATH`: `prior-auth.json`, `prior-agent.json`, `prior-session.json`, `prior-prompt-response.txt`, `prior-bound-session.json`, `prior-history.json`, and `prior-events.json`. The append-only journey log contains the corrected logical ID on subsequent actions; its initial create-session row retained an envelope-reading mistake.

This established a genuine old-runtime conversation before the upgrade walk. The later replay and native-load observations below used this same logical session and lab; the restart subsequently removed the temporary installation.

### Native operation and browser selection

- Native CLI 1.5.9 reused the existing login through the public remote readiness probe. Hydrogen `xhigh` was acknowledged as `thought_level=xhigh`; the unrestricted session acknowledged `mode=auto`, without injected `yolo`.
- A public `max` request returned HTTP 422 / `reasoning_effort_unsupported` and retained the effective `xhigh`.
- Default MCP transport delivered `compozy__workspace_info` successfully. An authorized native write also succeeded.
- Headed Chromium under Xvfb verified Settings command/auth ownership and absence of credential inputs, including a 375 px viewport.
- The headed picker verified keyboard selection, four Hydrogen stops through Extra high, three Argon stops through High, no duplicate thought control, narrow layout, an actual Hydrogen/xhigh submission, and persisted reply `AGY_BROWSER_7429`.
- The first picker driver used the wrong keyboard focus path; the second expected an extra punctuation mark absent from the requested reply. A headed reread verified the already-persisted result without submitting again. These were harness failures, not product fixes.
- Receipts: `native-auth.json`, `native-bound-session.json`, `native-reject-max.json`, `native-tools-and-write-verdict.json`, `native-browser-settings-verdict.json`, and `native-browser-picker-verdict.json`. Reasoning frames are excluded from acceptance evidence.

### Permission enforcement: failed default and failed plan rewalk

- The launch-only build wrote `agy-denied.txt` under `approve-reads` without a host permission request. This is a safety failure, not a successful denial.
- Corrections `654a95641` / `deb6d04bf` selected advertised `plan` and refused missing/unsafe mode selections. Their deterministic tests passed, but the live enforcement premise was false.
- The integrated binary (`native-current-build.json`) acknowledged `mode=plan` under `approve-reads` in `sess-f7e19ce5a1b6858e`, yet native `write_file` created fresh target `agy-denied-plan-7431.txt`. Both failed targets and receipts were retained in the lab until the restart removed it.
- Historical failure receipts: `native-denial-verdict.json`, `native-readonly-bound.json`, `native-readonly-response.json`, and `native-readonly-verdict.json`. These failures were not relabeled successful enforcement.
- Remediation in `8d501bba2` removed the disproven plan-based enforcement and refuses native `agy acp` execution below `approve-all` before spawning. It never silently widens permissions; promptless inspection and authored custom/standalone routes retain their behavior.
- The final committed binary returned HTTP 422 for `sess-4ba1e6e08de039f0`, retained `effective_permissions=approve-reads`, left `runtime.status=unbound`, reported no processes, and did not create fresh target `agy-denied-before-launch-7434.txt`. The two earlier failed-write targets still existed at that observation. Historical receipt: `native-policy-refusal-verdict.json`.
- This proves safe refusal, not native read-only execution. Always-on agents default to `approve-edits`, so their native AGY wakes are refused too unless the operator has explicitly configured a supported policy/runtime.

### Cancellation and process ownership

- Event-driven observation proved the native command was actually running before cancellation. The API returned `canceled` for `turn-042a58da0dbf0681`; a cooperative, verified `session.turn_quiesced` receipt followed, and the next prompt returned `AGY_USABLE_AFTER_CANCEL`.
- A stronger driver assertion failed because the native detached task remained alive after turn quiescence. This original result is retained in `native-after-cancel-verdict.json`; it must not be relabeled process cleanup.
- Connected-code review found that turn cancellation does not reap native task trees: `internal/session/stop_ladder.go` verifies turn quiescence, while `RT-session-processes` reserves tree termination for an explicit process/stop/archive operation. The approved native spec requires cancellation plus a usable follow-up, not implicit reaping at turn end.
- A separate walk observed the native task tree in `sess-04ed8efb4422e710`, obtained `canceled`, then explicitly terminated the exact process identities through the public process operation: root `terminated`, Python member `already_exited`, subsequent process list empty, follow-up `AGY_USABLE_AFTER_EXPLICIT_CLEANUP`.
- An immediate event read raced durable publication. A later independent read confirmed one persisted quiescence receipt without another task or prompt (`native-cancel-cleanup-confirmed.json`). This is explicit task cleanup, not implicit turn-end reaping. The earlier `nothing-in-flight` attempt is not cancellation proof.

### Lossless recovery and clean-home probe

- The first actual old-session resume retained incompatible `mode=yolo` and refused startup, but HTTP exposed only `500 Internal Server Error`. That failed public boundary is retained as history (`native-continuity-refusal.json`).
- After repair, the final binary returned bounded HTTP 422 reselection guidance, preserving native ID, accepted route, recorded options, creation identity, and history. Runtime-clear alone still refused; complete runtime-set confirmed revision `2`.
- The same logical session then returned `AGY-CONTINUITY-7429 AGY_NATIVE_REPLAY_7432` on a fresh native ID `b19ad5df-3e18-4b6b-a07c-d58f0ccd99c8`; creation identity stayed unchanged. This was transcript replay, not native restoration or private-history import (`native-continuity-verdict.json`).
- A subsequent verified stop and prompt retained that new native ID, logged successful `session_load`, and returned `AGY-CONTINUITY-7429 AGY_NATIVE_RESTORED_7435` (`native-restoration-verdict.json`). These single-use continuity scripts were not rerun.
- An implementation-build local ACP auth probe with an explicitly clean HOME/XDG environment returned `needs_login` / `provider_not_authenticated`, without invoking login or copying credentials (`native-clean-home-auth.json`, `native-clean-home-verdict.json`). Its first driver assertion assumed the remote response envelope; a reread validated the flat local result.
- A separate Google consent/login-cancellation walk and non-Linux execution remain unverified.
- The final committed binary's remote readiness was `authenticated`, clean-HOME local readiness was `needs_login`, and live catalog refresh completed (`native-final-auth.json`, `native-final-clean-home-auth.json`, `native-final-catalog.json`). A later full old-history/catalog comparison did not complete before restart; no byte-for-byte final-history or complete final-catalog verdict is claimed from it.

## Focused Engineering Verification

External-peer tests are separate from live Google evidence. Earlier results below were reported before restart; interrupted checks are not treated as successful.

- `bun install --frozen-lockfile`: exit 0; no dependency or lockfile change.
- `bunx turbo run build --filter=./web`: exit 0, five successful tasks. The existing frontend reports chunks over 500 kB; no bundle-performance change is in scope.
- `agy --version`: `1.5.9`; `agy acp --help`: exit 0, native subcommand present.
- Browser preparation found installed Python Playwright, Chromium under `/ms-playwright`, and Xvfb. The bootstrap's missing default drivers do not by themselves prove the browser journey blocked.
- Markdown files are ignored by the repository formatter (`Expected at least one target file`); whitespace and local links are checked separately.
- Launch slice integrated as local commit `031a13703`; the independent focused run passed `internal/config`, `internal/acp`, `internal/runtimemcp`, `internal/cli`, `internal/api/core`, and `internal/settings`. No full gate or E2E run was requested.
- Catalog/daemon slice integrated as `3edb8010d`; focused per-model and context-isolation checks passed.
- Independent root-filtered Turbo run passed four affected Web suites / 182 tests, with its pipeline codegen check.
- The combined recovery/read-only focused run passed `internal/acp`, `internal/session`, and `internal/config`. Pinned golangci-lint 2.15.0 reported zero new issues in those packages and `internal/testutil/acpmock`.
- The earlier five-file Go test-shape comparison reported 33 baseline findings and no additional findings; its original scratch receipt was lost. The broader post-restart comparison below supersedes it for final review.
- Final pre-restart independent checks passed six affected Go packages and pinned golangci-lint 2.15.0 reported `0 issues` before the review-only corrections. Worker-reported site, Web typecheck, fixture, and formatting checks also passed. Repository-wide gates and E2E remain CI-owned and were not run locally.
- Independent review found two P3 items: distinguish `native_route_unverified` from a removed `accepted_route_missing`, and test the resolved-command refusal plus lossless refused resume at the real daemon/process boundary. Both are addressed in the canonical resume and native-entry suites; post-restart focused results are recorded separately below.

### Post-restart local review

Persistent receipts are under workspace-root
`.compozy/tasks/agy-native-acp/evidence/final-review-20261117/`; corrected results use
`corrected-review/` and retain source hashes. The completed daemon checks are also recorded under
`evidence/post-restart-20261117/`. No result below replaces lost live acceptance evidence.

| Check | Result |
| --- | --- |
| `TestAgyACPBuiltInEntry`, real daemon/SQLite/ACP subprocess, integration tag, race enabled | Passed twice: 22.204 s and 22.046 s; the daemon test source is unchanged since those runs. |
| `TestResumeAcceptedRouteAffinity`, `TestResumeRouteChangeRebuildsEffectiveOptions`, `TestResumeReplayFallback`, race enabled | Passed after correcting two accidentally swapped test expectations: 11.285 s. Both earlier failed executions remain recorded and are not counted as passing. |
| `TestNativeAgyACPCommand`, race enabled | Passed: 1.046 s after giving each command its canonical `Should` subtest; no assertions removed. |
| Pinned golangci-lint 2.15.0, affected session/daemon packages with integration tag and config package, new-from baseline `1141d8640` | Both focused runs passed with `0 issues`; receipt hashes match the committed files. |
| Read-only test-shape comparison, all 16 changed Go test files | 74 current findings versus 81 baseline: 73 inherited findings and one documented helper-process false positive. The new config test-shape violation was repaired. |
| Changed QA scenarios, charters, and report | Four scenarios materialize through the canonical tracker helper; charter YAML/references, report links, and whitespace checks pass. This is documentation validation, not strict acceptance-evidence audit. |

The heuristic's only added finding is `TestAgyACPEntryPeer`, a raw-stdio subprocess entry rather than
a behavior case. Its existing source comment explains why assertions live in `TestAgyACPBuiltInEntry`
and `t.Run` is inappropriate there. `corrected-review/test-conventions-final.json` retains raw
baseline/current output and classification; only line-number shifts are normalized for comparison.
Unrelated baseline tests were not rewritten.

The focused race commands used `CGO_ENABLED=1 GOMAXPROCS=4`, `-p=4`, and `-count=1`. Daemon and
resume commands used the repository's modernc-only `-gcflags=modernc.org/...=-d=checkptr=0` exception,
not a global race/checkptr disable.

```bash
CGO_ENABLED=1 GOMAXPROCS=4 go test -race -gcflags=modernc.org/...=-d=checkptr=0 -p=4 -tags=integration ./internal/daemon -run '^TestAgyACPBuiltInEntry' -count=1 -timeout=180s
CGO_ENABLED=1 GOMAXPROCS=4 go test -race -gcflags=modernc.org/...=-d=checkptr=0 -p=4 ./internal/session -run '^TestResume(AcceptedRouteAffinity|RouteChangeRebuildsEffectiveOptions|ReplayFallback)$' -count=1 -timeout=180s
CGO_ENABLED=1 GOMAXPROCS=4 go test -race -p=4 ./internal/config -run '^TestNativeAgyACPCommand$' -count=1 -timeout=120s
```

Lint receipts retain the exact pinned executable and command: `run --concurrency=4 --timeout=5m
--new-from-rev=1141d864060678bfec1b75990973fba1b5a87bf1`, with `--build-tags=integration` for
`./internal/session/... ./internal/daemon/...`, and a separate run for `./internal/config/...`.

The original exact-manifest teardown command was attempted after restart and returned exit 1:
`TEARDOWN_ERROR=manifest not found`. `corrected-review/evidence-loss.json` and
`missing-manifest-teardown.log` record that failure, the absent original paths, and unavailable
strict audit/sanitization. No replacement manifest, broad recovery sweep, or `clean: true` result
was created.

## What Was Fixed

- Built-in native launch, per-model catalog/effort negotiation, and current public guidance.
- Safe pre-spawn refusal after native default and acknowledged plan both bypassed constrained host approval.
- Public HTTP 422 repair guidance without leaking commands/internal error chains.
- Lossless ambiguous-option refusal, explicit selection confirmation, transcript replay, and subsequent native restoration.
- Distinct durable recovery reasons and real daemon-boundary regression coverage.

All implementation and review commits are local only. The QA archive loss prevents final audited acceptance; successful deterministic checks do not replace it.

## Paper Cuts

The initial browser scripts had keyboard-focus and punctuation expectations corrected without a product change or duplicate successful submission. Bootstrap's default browser drivers were absent, but installed Python Playwright, Chromium, and Xvfb supported the completed headed walks.

## Runtime Errors Observed

- Bootstrap did not find its default browser drivers. Installed Python Playwright, Chromium, and Xvfb supplied the completed headed walks instead.
- Native default and plan both permitted a forbidden write; the advertised mode name is not security evidence.
- Old-option refusal was sanitized to a generic public HTTP 500, hiding the internally generated repair guidance.
- The server/container restart removed the temporary lab and in-flight terminal records before evidence closure. This is an external verification blocker, not an inferred provider regression.

## Human Verifications Needed

Clean OAuth consent/cancellation, non-Linux execution, and browser unknown/stale-source walks need their own evidence. None is inherited from spec research or claimed passed by deterministic tests.
The original acceptance archive must be recovered from an external backup or replaced by a
separately authorized acceptance run before an independently auditable QA PASS. This continuation
does not repeat successful provider work to manufacture replacement receipts.

## Decisions for a Human

Q2 was settled by the operator: preserve historical options whose provenance cannot be proved; only a complete explicit runtime selection confirms replacement intent. No new permission widening or clean OAuth consent was requested.

## Learnings

- Advertised options and acknowledged selections prove routing at the ACP boundary, not Google's internal compute budget.
- Transcript replay is not native-history import.

## Final Status

**BLOCKED-VERIFY.** Local implementation/review is committed and its focused checks passed, but the raw acceptance archive was lost on restart before sanitization, strict audit, and exact manifest teardown. The original teardown now fails because its manifest is absent; no `clean: true` receipt or audited PASS is claimed. A post-restart process/path inspection found no original lab/runtime directories or matching old lab processes; that is not the manifest teardown proof. All four implementation workers were independently read as stopped and verified. No push, PR, remote merge, new OAuth consent, or unrelated cleanup was performed.
