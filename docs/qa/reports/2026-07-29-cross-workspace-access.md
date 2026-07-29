# QA Run Report — 2026-07-29 — cross-workspace-access

- **Scope:** mode-anchored cross-workspace access across native tools, agent CLI, HTTP/UDS,
  audit readers, and Web session permalinks; targeted same-workspace canary; autonomous
  `northstar-pay` real scenario
- **Cadence tier:** targeted
- **Build:** `412ab876` plus QA-loop fixes, if any · **Environment:** fresh isolated
  `northstar-pay-20260729-124649-419333` lab; daemon/Web on manifest-assigned endpoints
- **Started:** 2026-07-29 · **Status:** targeted feature QA passed; strict autonomous playbook failed

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Ada | Runtime operator | desktop / wifi-fast / en-US | mode/seam matrix; same-workspace canary |
| Bruno | Delivery builder | desktop / wifi-fast / en-US | prompt outcomes; consent/audit interruption |
| Nia | First-time link opener | laptop / wifi-fast / en-US | foreign-session deep-link confirmation |
| Théo | Returning session user | desktop / wifi-fast / en-US | foreign-session pre-confirmation isolation |
| Sofia Mendes | Northstar delivery lead | playbook-defined | one autonomous operator kickoff |

## Flows in Scope

- `J-cross-workspace-access` — mode and session-consent decisions at every shipped seam.
- `J-open-foreign-session` — confirm-first canonical and short permalinks.
- `J-operate-workspace-context` — adjacent same-workspace resolution and binding canary.
- `northstar-pay` — full real-scenario playbook under one operator kickoff and runtime observation.

## Session Matrix & Results

| # | Charter | Journey / Scenario | Persona | Tour | Status | Issue | Fix commit |
|---|---|---|---|---|---|---|---|
| 1 | CH-cross-workspace-mode-seams | J-cross-workspace-access / ET-workspace-access-mode-matrix | Ada | Feature Tour | Fixed | BUG-20260729-coordination-cli-drops-agent-identity | 4ef8e8c |
| 2 | CH-cross-workspace-consent-audit | J-cross-workspace-access / ET-workspace-access-prompt-outcomes | Bruno | Interrupt Tour | Pass | | |
| 3 | CH-foreign-session-deep-link | J-open-foreign-session / ET-web-session-cross-workspace-confirm | Nia | Back-Button Tour | Pass | | |
| 4 | CH-foreign-session-deep-link | J-open-foreign-session / ET-web-session-deep-link-isolation | Théo | Back-Button Tour | Pass | | |
| 5 | CH-workspace-binding-canary | J-operate-workspace-context / ET-native-workspace-scope-isolation | Ada | Feature Tour | Fixed | BUG-20260729-nearest-workspace-case-alias | 4e81f17 |
| 6 | CH-workspace-binding-canary | J-operate-workspace-context / MS-workspace-resolution-chain | Ada | Feature Tour | Fixed | BUG-20260729-nearest-workspace-case-alias | 4e81f17 |
| 7 | northstar-pay playbook | autonomous startup scenario | Sofia Mendes | Scenario playbook | Fail | BUG-0028; BUG-20260719-autonomous-progress-unobservable (existing, out of scope) | |

Status legend: `Pending | Pass | Fixed | Skipped | Blocked (needs human verify) | Blocked (human decision)`

## Session Debriefs

- **Mode seams:** deny-all and approve-reads produced the same daemon-owned denial across native,
  CLI, HTTP, and UDS; approve-all crossed every tested seam. The corrected coordination CLI now
  carries validated session identity. Audit payloads remained scoped to the actor-home workspace.
- **Consent/audit:** once/session allow and reject answers behaved as declared, both operator answer
  surfaces agreed, timeout stored no grant, and stop/restart invalidated volatile consent. All four
  audit readers agreed on the same attributable decisions.
- **Foreign links:** canonical and short links showed the route-owned confirmation, survived
  back/forward/reload and a second tab, never fetched target detail before consent, and preserved the
  current workspace on cancel. Confirmation restored the target desktop and exact session.
- **Binding canary:** the five-tier CLI precedence walk, nearest nested CWD, unregistered-directory
  diagnostic, 11-entry catalog, and native same-workspace calls all passed after the case-alias fix.
- **Northstar Pay:** one Sofia Mendes kickoff activated the playbook and all 12 deterministic tasks
  and runs completed. Public logs contain 19 successful Network sends and the review catalog contains
  three real review records, but only one reached a verdict. None of the three declared disruption
  seeds was delivered and no resolved disagreement is evidenced. Independent public Task evidence is
  indexed because the existing observer integration still receives no runtime-owned progress rows.

## What Was Fixed

- `BUG-20260729-coordination-cli-drops-agent-identity` — coordination CLI methods now forward
  validated agent credentials through the agent-aware UDS transport. The red transport regression,
  full CLI race suite, and live deny/read/all replay pass (`4ef8e8c`).
- `BUG-20260729-nearest-workspace-case-alias` — nearest-root discovery now falls back to filesystem
  identity across existing path aliases while retaining lexical containment as the fast path. The
  red resolver regression, full workspace race suite, and live nested-root replay pass (`4e81f17`).

## Paper Cuts

- Chrome's primary CDP path required manual remote-debugging approval. The manifest-permitted
  `agent-browser` fallback completed the full Web charter and both browser sessions were closed.
- The first native audit search used `OR` as though the search input were a query language. The tool
  correctly performs literal substring search; the corrected `workspace.access_` read found 33 true
  events. This was operator misuse, not a product defect.

## Runtime Errors Observed

- Before `4ef8e8c`, agent CLI coordination reads arrived as the human operator and bypassed the
  workspace mode. HTTP/UDS controls remained correct.
- Before `4e81f17`, a case alias made nested CWD discovery fall through to the broad home workspace.
- The initial real-scenario observer exhausted its stall threshold while the playbook agents kept
  working. All 12 tasks later completed, but the journey log still had no runtime-owned task,
  session, or Network progress. This is another reproduction of existing
  `BUG-20260719-autonomous-progress-unobservable`, owned by `RT-073` outside this run's six selected
  feature scenarios. No second operator prompt masked it.
- The real-scenario controller did not deliver the three time-based disruption seeds. The partner
  status file and decision note explicitly confirm that no partner-timeout signal occurred. Two of
  three review records remain `in_review`, and no public evidence establishes a resolved agent
  disagreement. These facts keep the strict playbook verdict failed even though all 12 tasks and the
  cross-workspace feature charters completed.

## Human Verifications Needed

None currently. Browser-use required a local Chrome permission, but the explicit fallback removed
the verification dependency without reducing charter coverage.

## Decisions for a Human

No in-scope product decision. `BUG-0028` is reproduced at the collaboration-completion boundary and
`BUG-20260719-autonomous-progress-unobservable` remains an open P1 for the one-kickoff observer
contract; this run did not widen into either subsystem.

## Learnings

- Agent CLI parity depends on forwarding identity at the transport boundary; a command can otherwise
  look structurally correct while silently becoming an operator request.
- Registered-root containment must respect filesystem identity, not only lexical spelling, on
  case-insensitive and symlinked paths.
- The owner projection is sufficient to decide a foreign link. Network traces are the strongest
  proof that no foreign session detail entered a workspace-scoped cache before confirmation.
- Observer-owned Task comparisons can prove terminal truth, but they must not be relabeled as
  runtime-owned journey events; the missing projection remains a distinct known bug.

## QA Bootstrap

```yaml
qa_bootstrap:
  status: torn_down
  manifest: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/bootstrap-manifest.json
  lab_root: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab
  runtime_home: /var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-d97d70a4453d/runtime
  base_url: http://127.0.0.1:55292
  verification_evidence:
    - manifest health=fresh and reused_lab=false
    - isolated port 55292, UDS path, tmux-bridge socket, provider homes, and Web proxy validated
    - northstar-pay materialized with 11 agents, 12 tasks, 10 channels, and 3 disruption probes
    - behavioral charter has no placeholders and agent workspaces remain under project/
    - 12/12 deterministic playbook tasks and runs completed after one operator kickoff
    - 19 successful Network sends; 3 reviews observed, only 1 with a verdict
    - 0/3 declared disruption seeds delivered; no resolved disagreement evidenced
    - CLI race 1329/1329; workspace race 107/107; make lint zero issues
    - make test-e2e-runtime exit 0; make test-e2e-web exit 0
    - final make verify PASS: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/final-make-verify.log
    - four browser screenshots and cross-workspace-access-results.md indexed under qa-artifacts/qa
    - strict audit: C1-C10, C12-C16, C18 pass; C11/C17 retain factual playbook gaps
  teardown: /Users/pedronauck/dev/qa-labs/compozy-northstar-pay-20260729-124649-419333-lab/qa-artifacts/qa/teardown.json
```

## Final Status

- **Exit gate:** focused race suites, zero-tolerance lint, runtime E2E, browser E2E, and the final
  program-level `make verify` pass. The strict QA audit retains only the factual disruption and
  collaboration gaps; clean teardown remains the final live-lab gate.
- **Issues by user impact:** in-scope Blocks-Completion 2 fixed; existing out-of-scope
  Blocks-Completion P1 1 reproduced and still open.
- **Coverage:** 4/4 charters; 6/6 selected QA scenarios settled; autonomous playbook 12/12 tasks,
  1/2 required complete reviews, 0/3 disruptions, and 0/1 resolved disagreements.
- **Teardown:** PASS — `teardown.json` reports `clean: true`, `survivors: []`; registered daemon,
  Web, and observer PIDs are stopped.
- **Verdict:** FAIL — the targeted cross-workspace feature QA passes, but the required autonomous
  playbook does not meet its disruption, review-cycle, disagreement, or observability contract.
