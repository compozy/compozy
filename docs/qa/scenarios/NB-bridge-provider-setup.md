---
id: NB-bridge-provider-setup
area: NB
title: Set up, verify, and test bridge providers
persona: Tessa
journey: J-connect-bridge-provider
expected: Public Compozy surfaces generate a schema-valid Slack manifest; accept interactive or strict JSON setup without exposing secrets; require exactly one private-chat, ordinary-group, or forum-topic routing shape per Telegram bridge; reject credential-bearing upstream endpoints from provider configuration; return actionable pass, warn, fail, or skipped verification records without changing lifecycle state; make bare `compozy doctor` perform no live bridge checks while `compozy doctor --only bridge` opts in; register Telegram webhooks; send one real provider test message while preserving valid UTF-8 opaque target IDs, including URL-encoded `/` in the path ID, and accepting only literal `direct-send` or `reply` while rejecting explicit empty or null mode; and keep test-delivery dry-run only.
entry_points: compozy bridge manifest; compozy bridge setup; compozy bridge verify; compozy bridge send-test; compozy doctor --only bridge; HTTP and UDS bridge manifest, verify, webhook-register, and send-test routes
qa_status: untested
bug_ids: BUG-20260713-telegram-route-shapes
fix_status: deferred
retest_status:
fix_commits:
evidence: /home/pedronauck/dev/qa-labs/compozy-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/notes/bridge-charter-results.json; /home/pedronauck/dev/qa-labs/compozy-hermes-bridge-task-10-20260713-022226-583543-lab/qa-artifacts/qa/issues/BUG-0039.md;/Users/pedronauck/dev/qa-labs/compozy-qa-misc-network-goal-release-site-20260730-060405-932516-lab/qa-artifacts/qa
last_report: docs/qa/reports/2026-07-28-untested-full.md
overlaps: NB-025; NB-bridge-tool-progress; NB-long-bridge-replies; NB-provider-progress-rendering
---

An operator or agent configures a bridge through public Compozy surfaces, catches credential and webhook mistakes before enablement, and proves real delivery.

Added by the Hermes bridge Task 04 impact flag. Task 09 assigned it to `J-connect-bridge-provider` and the provider setup charters; Task 10 owns execution. Planning flag only; no QA session ran.

QA 2026-07-13: WhatsApp/Discord validation and Slack/Telegram setup/control surfaces worked, but guided Telegram routing requires group plus thread and rejects documented direct-message and ordinary-group shapes. `BUG-20260713-telegram-route-shapes` requires a structural alternative-route decision.

Phase D impact flag 2026-07-13: guided and strict-JSON Telegram setup now accept exactly one explicit private, ordinary-group, or forum routing shape per bridge. Bridge doctor checks are now opt-in through `--only bridge`, and warn/fail checks without provider remediation no longer claim to have passed. Status reset to `untested` for the next QA cycle; historical `BUG-20260713-telegram-route-shapes` evidence remains attached only to the broader one-instance alternative-shape contract. No QA retest ran.

QA impact 2026-08-02: real `send-test` delivery now preserves every valid UTF-8 opaque target
identity byte, routes URL-encoded slash IDs as one identity, and rejects lossy Unicode plus
delivery-mode aliases, case changes, whitespace variants, explicit empty strings, and null before
transport. Historical evidence is retained; status reset to untested pending a fresh public-surface
walk.

QA impact 2026-08-06: confirmed gateway bindings now supply bridge webhook registration URLs and
ingress health while unbound external proxies remain unchanged. Status remains untested;
remote-gateway Tasks 08–09 own the walk.
