---
id: RT-model-catalog-cold-open
area: RT
title: Cold model catalog opens and repairs missing provider rows
persona: Sol
journey: J-17
expected: On the first selector open after daemon start, persisted rows are immediately usable; if an allowed provider has no rows, Web requests one aggregate refresh and rereads the catalog once, while provider probes remain daemon-owned background work. Newly advertised models appear without a code update, failed refresh keeps rows stale, and shutdown joins or cancels work cleanly.
entry_points: onboarding default-model selector; agent runtime selector; session composer RuntimeSelector
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-cursor-onboarding-runtime-defaults-retest-20260828-171621-219738-lab/qa-artifacts/qa/notes/cursor-defaults-retest-evidence.md; /Users/pedronauck/dev/qa-labs/compozy-integrated-terminal-rebase-20260828-201516-678087-lab/qa-artifacts/qa/screenshots/onboarding-runtime.png; docs/qa/reports/2026-08-28-integrated-terminal-rebase.md
last_report: docs/qa/reports/2026-09-12-pr-630-634-integration.md
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
