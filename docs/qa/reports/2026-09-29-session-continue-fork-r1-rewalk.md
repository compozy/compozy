# QA Run Report — 2026-09-29 — session-continue-fork, review round 1 re-walk

- **Scope:** only the evidence invalidated by the review-round-1 fixes (`reviews-001/review.md`; W1–W4 in `memory/peer-review.md`), plus the still-owed Web leg of `RT-conversation-rewind`. CLI/API: `ET-cli-session-continue` (committed-child error output, `<code>: <message>`), `RT-session-derive-retry` (deleted-source replay, post-commit `child_session_id`), `RT-session-derive-native-fork` (spot check, capability-based wording). Web: `ET-web-session-continue` (explicit normal speed, reopened dialog measures, committed-child "Open new session"), `ET-web-session-fork-from-here` (reopen measures, fresh fences), `RT-provider-error-handoff` (no live duplicate), `RT-agent-detail-runtime-live-edit` (fallback chain survives Web edits), `RT-conversation-rewind` (busy gate, rewind from the message row). VC re-capture where rendering changed.
- **Cadence tier:** targeted
- **Build:** branch `continue-fork` head `62bdd8054`; lab daemon and acpmock driver built from the tree; Web built with `vite build --outDir <lab>/web-dist` and served through `COMPOZY_WEB_DIST_DIR`. The Web was rebuilt after each of the two fixes below, which are uncommitted.
- **Environment:** isolated lab `compozy-session-continue-fork-r1-rewalk-20260929-041438-936027-lab` (own `COMPOZY_HOME`, UDS, HTTP 127.0.0.1:62497, Playwright driver on 62498), acpmock only, apart from one cursor agent that was only configured (no prompt, no provider cost). The agents mirror B2's lab (`alpha`, `beta`, `bad-model`, `fork-web-agent`, `fork-native-agent`, `handoff-agent`, `lab-runner`, `route-agent` with two routes) plus `fast-cursor` (cursor `composer-2.5`, `speed: fast`). Headless Chromium 1440×900.
- **Evidence:** `docs/qa/evidence/2026-09-29-session-continue-fork-r1-rewalk/` (gitignored lean copies, including `journey-log.jsonl`); lab scratch under `<lab>/qa-artifacts/ev/`.
- **Started:** 2026-09-29T04:14Z · **Status:** closed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Rafa | operator on the CLI/API | desktop / local / en-US | CLI continue, native fork, handoff |
| Théo | interrupt-minded operator | desktop / local / en-US | retry after deletion, rewind |
| Bruno | desktop builder | desktop 1440×900 / local / en-US | Web continue/fork/agent edit |

## Session Matrix & Results

| # | Journey / Scenario | Persona | Status | Issue | Fix commit |
|---|---|---|---|---|---|
| 1 | J-15 / ET-cli-session-continue (errors, committed child) | Rafa | Pass | | |
| 2 | J-15 / RT-session-derive-retry (steps 7–8) | Théo | Pass | | |
| 3 | J-15 / RT-session-derive-native-fork (spot check) | Rafa | Pass | | |
| 4 | J-14 / ET-web-session-continue (speed, reopen, committed child) | Bruno | Fixed | BUG-20260929-derive-refusal-below-fold | uncommitted |
| 5 | J-14 / ET-web-session-fork-from-here (reopen, fences) | Bruno | Pass | | |
| 6 | J-14 / RT-provider-error-handoff (live notice) | Bruno | Pass | BUG-20260928-provider-error-notice-live-duplicate (retest pass) | |
| 7 | J-31 / RT-agent-detail-runtime-live-edit (fallback-chain leg) | Bruno | Pass | | |
| 8 | J-rewind-conversation / RT-conversation-rewind (Web leg) | Théo | Fixed | BUG-20260929-rewind-draft-lost-on-row-unmount; BUG-20260929-rewind-offered-after-rewind (open, pre-existing) | uncommitted |
| 9 | E2E-005 / VC-22 recapture, VC-15 re-render check | — | Pass | | |

## Session Debriefs

### CLI/API — Rafa, Théo

- `session continue` usage errors exit 2 with the documented text. Daemon errors print `error: <code>: <message>` and exit 1 (`agent_not_found`, `session_not_found`); `-o json` keeps `code`.
- Post-commit failure (`bad-model` + `--message`, 422): the CLI prints the daemon error, then `session sess-… was already created; open it with \`compozy session status sess-…\`, or rerun with --idempotency-key <key> to get it back`. `-o json` and HTTP carry `child_session_id`. The same key reuses that one child.
- Deleted-source replay: continue A→B with key K, remove B, remove A, then rerun the same command from the workspace cwd → `Replayed yes`, `Child deleted yes`, origin `alpha`; HTTP `200`. Nothing was created. Preview and fork leave the source `meta.json` sha unchanged.
- Native fork: `fork-native-agent` (`acp_caps.supports_fork_session: true`) → `native_fork` pending → loaded on the child's first prompt; `alpha` (fork false) → replay. There were no clone ids in the source events. The scenario said `acp_caps` is on `session status -o json`; it is on `GET /api/sessions/<id>`, and the wording is fixed.
- Paper cuts: the post-commit 422 has no top-level `code` (only `diagnostic.code: model_unavailable`), so the human line has no `<code>:` prefix and shows the internal error chain (a retry can append an adapter `stderr=` tail). `_dx.md` examples print `Error:`; the CLI prints `error:`.

### Web — Bruno

- Explicit normal speed: for `fast-cursor` the dialog showed "fast speed requested". With the switch off, the POST carried `speed: "normal"` and the child meta persisted `normal` (W3 #11 confirmed live).
- The reopened Continue and Fork dialogs read `Measuring…` with the primary disabled while their own preview is held. The fork POST then used the fresh fences (`expected_max_sequence: 18`) (W3 #14 confirmed live).
- Committed child: the refusal and **Open new session** rendered below the fold of the default 604 px window, so pressing Continue appeared to do nothing → fixed. Afterwards the offer is visible and opens the child window with the pill "Continued from alpha" (W2 #9 confirmed live).
- Live provider notice: one notice while the stream is live and one after reload (W3 duplicate fix confirmed).
- Agent edits: an Overview runtime change and a Settings → Instructions save both send `fallback_chain`. `GET` and `AGENT.md` keep both routes with fingerprints (W3 #8 confirmed live).
- Rewind: the busy gate disables Fork and Rewind together while an API-started turn runs and releases both after cancel. Rewind cut the session correctly, but the rewound prompt never reached the composer → fixed and re-walked (draft "Second step" restored, sent, answered on the same session).
- Surprise: after a rewind followed by new turns, "Rewind to here" stays offered, but the daemon refuses it by design; filed as pre-existing (open).

## What Was Fixed

### BUG-20260929-derive-refusal-below-fold
- **Symptom:** after a post-commit 422, the refusal and "Open new session" sat below the dialog's scroll fold; nothing visible changed.
- **Fix:** `web/src/systems/session/components/session-derive-committed-child.tsx`: `SessionDeriveSubmitOutcome` renders the refusal plus the committed-child offer and scrolls them into view (`block: "nearest"`) when they appear. Both dialogs use it (`session-continue-dialog.tsx`, `session-fork-dialog.tsx`), and the test ids are unchanged.
- **Regression test:** `session-continue-dialog.test.tsx` "Should bring a post-commit refusal into view when it appears" (mutation check: fails without the ref).
- **Retested:** visible without scrolling; Open new session opens the child.

### BUG-20260929-rewind-draft-lost-on-row-unmount
- **Symptom:** Web rewind succeeded (`200`, `draft_text`), but the composer stayed empty and no toast appeared.
- **Root cause:** the hook aborted its request on unmount. The live transcript drops the rewound row before the POST response is read, so the request showed `ERR_ABORTED` and the prefill and runtime reset were skipped. Pre-existing since #310.
- **Fix:** `web/src/systems/session/hooks/use-session-rewind-message-action.ts`: only dismissing the dialog aborts; an accepted rewind always resets the runtime and prefills the draft.
- **Regression test:** `session-chat-runtime-provider.test.tsx` "Should restore the rewound prompt as the draft after its message leaves the transcript" (response held until the row is gone; failed before the fix). The harness gained a `/rewind` route.
- **Retested:** composer "Second step" after the rewind; sending it answered on the same session.

Checks after the last change: `bunx vitest run src/systems/session src/components/assistant-ui` 97 files / 1209 tests passed; `tsc --noEmit` clean; `oxlint --deny-warnings` and `oxfmt --check` clean on the changed files. `make gate` was not run by this pass: a concurrent run owns it, and it started before these Web edits, so its result does not cover them.

## Visual Contract

- **VC-22:** recaptured on the live state (one notice). `comparison.json` was regenerated with the verdict and authorized differences kept, and `review.md` was updated; `validate-visual-contract.mjs` → PASS.
- **VC-15:** re-rendered after the shared refusal wrapper; layout and copy match B2's capture (only lab bytes differ), so the bundle was not replaced. Capture: `…/VC-15-implementation.png`.
- **VC-06..VC-14:** no state in these bundles renders the committed-child block, and first-open measuring is unchanged, so they were not recaptured. The committed-child offer has no board specimen.

## Paper Cuts

| Persona | Where | Felt | Sharpness | Outcome |
|---|---|---|---|---|
| Rafa | CLI post-commit 422 | "The error is an internal chain with no code" | dull | watching (422 has only `diagnostic.code`) |
| Bruno | Continue to an unbound child | "Where do I see that the child really is normal speed?" | dull | watching (`Speed --` until bind) |
| Théo | Second rewind | "Refresh doesn't help" | sharp | filed BUG-20260929-rewind-offered-after-rewind |

## Decisions for a Human

- **BUG-20260929-rewind-offered-after-rewind** (pre-existing, outside this feature): recommend giving the refusal a code (`rewind_target_invalid`) with a copy hint to fork instead, then exposing rewind eligibility so the action is not offered.

## Final Status

- **Verdict:** PASS. All 9 rows are terminal: 7 Pass, 2 Fixed-and-retested. Findings: Trust-Damage 2 (fixed), Friction 1 (open, pre-existing, escalated).
- **Limitations:** a committed child surviving a post-commit failure and the native snapshot drift race cannot be reached from public surfaces; they stay owned by `TestDeriveCommitBoundaries` and `TestForkNativeGate`. `RT-agent-detail-runtime-live-edit` was walked for the fallback-chain leg only. Strict audit: C14 (local gate evidence) is open, because `make gate` must be rerun on a tree that includes these Web edits.
- **Teardown:** `teardown.json` `clean: true` (see `memory/peer-review.md` § Round 1 — QA re-walk).
