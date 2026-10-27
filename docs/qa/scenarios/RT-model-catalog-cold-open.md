---
id: RT-model-catalog-cold-open
area: RT
title: Cold model catalog opens and repairs missing provider rows
persona: Sol
journey: J-17
expected: On the first selector open after daemon start, persisted rows are immediately usable; if an allowed provider has no rows, Web requests one aggregate refresh and rereads the catalog once, while provider probes remain daemon-owned background work. Newly advertised models appear without a code update, failed refresh keeps rows stale, and shutdown joins or cancels work cleanly.
entry_points: onboarding default-model selector; agent runtime selector; session composer RuntimeSelector
qa_status: blocked-verify
bug_ids:
fix_status:
retest_status: blocked
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-cursor-onboarding-runtime-defaults-retest-20260828-171621-219738-lab/qa-artifacts/qa/notes/cursor-defaults-retest-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/onboarding-runtime.png; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-09-16-claude-fable-reasoning.md
overlaps: ET-web-runtime-selector-minimal-slider; RT-068; RT-072
---

QA impact 2026-08-27: catalog reads no longer perform sequential live provider probes on the request
path. The daemon returns persisted entries immediately and owns the asynchronous refresh lifecycle.

QA impact 2026-08-27: live catalogs now persist option descriptors and private bindings by execution
context and refresh on TTL plus periodic cadence. Reset for cold read, stale failure, new-row, and clean
shutdown evidence.

QA 2026-08-27: after publishing and then failing a same-source synthetic Cursor model feed, a fresh
daemon process returned the persisted `available_stale` row in 0.02 seconds. The read did not wait for
the failing provider probe, and a later successful forced refresh replaced the stale generation.

QA impact 2026-08-28: the first Web read now performs one deduplicated aggregate refresh and reread
when a configured provider such as Cursor has no persisted rows. Reset for a fresh-onboarding walk
that proves the models appear without pressing Reload.

QA 2026-08-28: pass. A fresh isolated onboarding opened the model picker once and immediately
showed Cursor Agent, Grok 4.5, Grok 4.6, and GPT-5.6 Terra without pressing catalog refresh.

QA 2026-09-09: pass for the changed catalog/reasoning journey. Fresh native discovery exposed Astra through Ultra, Luna through Max, Grok 4.5 through High and Grok 4.6 through Extra high. Saved runtime survived reload/restart. Native Cursor and Codex prompts completed; unsupported Cursor combinations returned 400 without changing the selection. See the current report for captures, exact runtime evidence and limits.

QA impact 2026-09-12, issue #627: overlay discovery must remain account-scoped. The added checks
below passed in the September 12 integration report, using native CLI/API evidence plus the
existing ACP/SQLite isolation suite. Earlier evidence covers unchanged selector interactions.

1. Create a provider overlay with `runtime_provider = "claude"` and its own account command.
2. Refresh/list that provider and inspect `provider_live:<overlay-id>` status. Only its advertised
   models should appear; a runtime-family seed alone must not create a live model.
3. Curate a returned logical ID and start a session. The overlay command must receive its advertised
   transport alias while the persisted session keeps the overlay provider and logical model ID.
4. With two accounts advertising different models, verify that neither catalog or session admission
   accepts the other account's live bindings.
5. Change the overlay command, apply its reported restart requirement, and refresh; prior account
   rows must not remain live under the new execution identity. Remove the overlay through Settings and confirm it leaves scheduled discovery
   and catalog projections without restarting the daemon.

6. Recreate the removed overlay with the same command while its discovery is unavailable. No
   previously removed live models may reappear, including stale entries from other profiles/workspaces.
7. For a Cursor-family overlay with an explicit discovery command, set an advertised logical runtime
   model on active and stopped sessions. Reject raw aliases before persisting a selection revision.

QA 2026-09-12: pass for the overlay slice. Settings live-add, native Claude discovery/auth, curation,
agent creation and a completed prompt preserve overlay/logical IDs with the private `haiku` binding.
Deletion/recreation with the same unavailable command leaves zero cached models; restoring the
command and forcing refresh recovers five models. Applying a changed offline command invalidates
the prior identity. Cursor active/stopped selection succeeds; foreign logical IDs and raw aliases
fail without changing revision 2. Two distinct account catalogs and private bindings are checked
by the existing ACP-subprocess/SQLite integration suite, not claimed as two vendor logins. See the
report for evidence and the API snapshot regression discovered and repaired during the walk.


QA impact 2026-09-16, issue #655 (verification pending): use the installed Claude adapter in an
isolated daemon and select the exact Fable 5.1 identity from the full list and an existing recent or
favorite row. Compare cold discovery, persisted rows and a resumed session. Record the advertised
model-specific thought levels, choose an effort, submit a real prompt and confirm the effective
runtime plus ACP current value. Switch models while preserving a valid effort and clearing an invalid
one; exercise rapid selection, reload and queued-prompt snapshots. A model with unconfirmed metadata
must not claim provider-managed effort. Failed discovery must preserve prior rows and recover after
refresh. Keep distinct exact model versions and account/profile/workspace scope unchanged.

QA 2026-09-16, issue #655: Fresh discovery produced exact Fable 5.1 and model-specific levels. Persisted rows survived restart; failed refresh retained prior valid rows as stale. Successful refresh recovery remains blocked. The installed adapter independently confirmed the account Fable usage limit; a later application attempt also reported native authentication resolution failure. No selected intent was reported as applied. See the current report for public rendered evidence and the remaining recovery steps.


QA impact 2026-10-11 (verification in progress): Create Claude and Cursor sessions using a recently discovered model in the same provider/profile/workspace context. Confirm no forced discovery is needed for non-stale rows aged between five and ten minutes; repeat at ten minutes, beyond ten minutes, and with stale discovery and verify refresh. Session creation retains a bounded 90-second default and explicit shorter deadlines still cancel. Evidence and exact verification scope: [startup/cache/notification report](../reports/2026-10-11-startup-cache-notifications.md).

QA impact 2026-10-16, issue #740: With `cursor-agent` installed but signed out,
refresh the session runtime selector while Codex/Claude are healthy. Confirm a
calm Cursor-attributed sign-in hint, no global red subprocess error, and that
healthy providers and cached/curated models remain selectable. Compare
`compozy provider models status cursor -o json`, HTTP/UDS status, native
`compozy__provider_models_status`, and Settings: failed live discovery carries
`failure_reason=auth_required`; redacted `last_error` remains diagnostic-only.
Repeat with stale cached rows, a timeout, and a successful recovery. Rendered
Storybook and targeted Go/Web checks verify presentation/classification; no live
provider login or full-daemon lab is implied by that evidence.

Issue #740 evidence: [targeted verification and rendered before/after report](../reports/2026-10-16-issue-740.md). Live vendor login/recovery remains outside the authorized local validation scope.

QA impact 2026-10-16, post-merge CI repair (#745): repeat the session selector search
with at least twelve providers reporting multi-line discovery failures and one healthy
model. Provider guidance must remain scrollable without collapsing the model viewport.
Click the healthy model with a real pointer, confirm its reasoning controls remain
usable, and preserve the provider/model/effort in the created session. The existing
`ProviderAuthRequired` browser story owns the hit-target regression; the two
`session-provider-override.spec.ts` journeys own persisted runtime truth. A local
render reproduced an 8px model viewport and an intercepted click before the fix;
the bounded guidance region is verified through the same rendered story after the fix.
Full daemon and desktop validation remains CI-owned.

QA impact 2026-10-19, issue #827: provider guidance moved out of the selector body. With one or
more providers reporting discovery problems, open the session composer selector and confirm the
model list starts directly under the provider chips. A warning mark sits before refresh; its
popover lists each provider and message. Escape or a press elsewhere in the selector closes only
that popover and returns focus to the mark; a second Escape closes the selector. With every
provider healthy, no mark renders. Targeted Web tests and rendered Storybook (light/dark, one and
many issues) verify presentation and dismissal; a live signed-out provider walk remains CI/QA-owned.

QA impact 2026-10-18, opencode-model-discovery Task 03 (fixture proof only; live legs unverified): with models.dev disabled and no preseeded OpenCode rows, open the session Runtime selector and select provider `opencode`. Confirm actual native discovery runs (`--version`, then v1 `models --verbose` or workspace-scoped v2 `api --standalone GET /api/model`), select `openrouter/acme/coding-default` with effort `high`, and complete a first prompt: `runtime.effective` must show the selected provider/model/effort and the fixture's discovery/ACP `exe=` markers must agree. Restart (fresh bind) and complete a second prompt with compatible intent. Failure legs: an incompatible selection fails before inference with no silent substitution; a native startup failure leaves the npm canary untouched and joins owned processes. Live vendor, npm acquisition, and private-service lifecycle legs remain unverified (Task 05 / V-001).

QA 2026-10-18, opencode-model-discovery Task 05 (live, build from Tasks 01–03 diff; installed v2.0.32; models.dev disabled; browser unavailable so CLI equivalents): L1 pass — `provider_live:opencode` 92 live rows; `opencode/beam-free` + `low` completed both first and post-restart prompts with agreeing effective runtime. L3 pass — forced refresh/recovers, timeout→stale→fresh. L4a/L3-partition pass — bogus explicit discovery fails with no native fallback; second-install selector refreshes and launches from its own path. L5 pass — owned ACP exits on stop, pre-existing shared service intact, waiter-kill leaves daemon consistent with no orphans. L6 pass — CLI/HTTP/UDS/native-tool agree on IDs + `provider_live:opencode`. Blocked-external: L4 npm end-to-end (npm prefix selected and package v1.19.3 acquired, but v1 listing rejects the v2-created global db in the authorized HOME; product fails truthfully with no fallback) and L2 v1 (no pinned v1 binary). Report: [2026-10-18 task05](../reports/2026-10-18-opencode-model-discovery-task05.md).

QA impact 2026-10-18, selector follow-up: keep an explicit OpenCode curated set containing only a
deprecated model. Confirm `view=all` still contains native live rows with `curated=false`. Open a
session's Runtime selector, click OpenCode with an empty search, and verify all discovered models
appear and can be selected with their exact IDs. All models retains curated browsing; switching
providers must not show foreign rows or bypass authentication. Repeat with v1 and v2 executable
fixtures and complete the existing prompt journey without changing operator configuration.

Selector retest: **pass**. Live `make dev` Chromium replay rendered all 92 available v2.0.32
models (174 total catalog rows), with an empty search and no disabled live rows. Keyboard edges
and Escape focus return passed. Daemon-served OpenCode v1→v2 fixture selection/prompt/restart
passed; the complete provider-override spec passed 4/4. See the selector follow-up in the
[2026-10-18 report](../reports/2026-10-18-opencode-model-discovery-task05.md); other live-vendor
and platform gaps above remain unchanged.

QA impact 2026-10-18, OpenCode list metadata: OpenCode rows show a muted native-provider label
after the model name, and its catalog group sorts by provider then model display name ascending
in browsing and search. Missing/blank/null provider metadata produces no label or exception and
does not hide the model. Exact selected IDs and pinned recent/favorite order remain unchanged;
Codex, Claude, and Antigravity retain their prior labels and ranking. Verify the v1→v2 executable
fixture journey and inspect narrow/wide rendering for overflow.

Metadata retest: **pass**. The existing selector suite passed 104/104, including null/blank
provider handling, provider/display-name order, exact IDs, and unchanged Codex/Claude/Antigravity
ranking. The daemon-served provider-override suite passed 4/4 with v1/v2 label assertions.
1280×720 and 390×844 captures show the labels on one line without picker overflow. This is
executable-fixture/browser evidence, not a new live-vendor inference claim.

QA impact 2026-10-22, issue #549: force Claude or Cursor discovery to fail with no prior live
rows, then with cached bindings. Curated rows must remain inspectable with `available=false`
and `availability_state=unknown`; the selector must disable them and show
`live catalog unavailable: <last refresh error>`. Direct curated-Claude session creation must
report that diagnostic before launch. Recover discovery and verify the same logical model becomes
selectable and resolves its private transport binding. Repeat with a Claude overlay to verify
account isolation. Cold ACP discovery has its own 60-second default; a configured 90-second
provider discovery timeout must not be truncated by the models.dev HTTP timeout.

Focused unit coverage owns failure/recovery, picker mapping, and timeout wiring. The existing
ACP subprocess/SQLite overlay integration scenario owns the real process/storage boundary.
A new live-vendor Windows walk and rendered browser replay remain CI/QA-owned.

Review follow-up: after restart, test bindings aged nine minutes, exactly ten minutes, one day,
with a future timestamp, and with no timestamp. Only the nine-minute binding remains reusable.
Claude aliases (`sonnet`, `haiku`), `default`, and raw IDs outside the built-in curated set remain
selectable without a binding, including overlays. A long discovery error must remain in provider
guidance while the row badge stays `Unconfirmed`. Verify aggregate refresh grants every sequential
probe its complete deadline and live HTTP discovery never uses models.dev's shorter client timeout.

Review follow-up 2026-10-23: with discovery failing and no live bindings, Claude and Cursor
admission retain `model_unavailable` and the discovery diagnostic without launching a provider.
The existing HTTP negotiation mapping remains 422. The focused session admission suite passed;
final-head CI owns the HTTP/integration verification.


QA impact 2026-10-27 (session responsiveness): model option and source-issue projections
now retain their collections across unrelated renders. Change the mounted selector's
provider order/authentication and authoritative model rows; the visible options,
disabled state, stale state and provider guidance must update immediately. Preserve
one aggregate read, initial missing-provider refresh, explicit forced refresh and
failure/recovery semantics. The existing hook suite owns these transitions; the
agent create and session provider-override browser journeys own the daemon/UI slice.
Current-head CI evidence is recorded in PR #983. This change does not claim a new
live-vendor authentication or discovery walk.
