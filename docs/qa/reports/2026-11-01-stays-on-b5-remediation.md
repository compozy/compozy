# QA Run Report — 2026-11-01 — Stays-on B5 remediation

- **Scope:** User agent authoring under All projects, profile history isolation, and missing-provider recovery.
- **Cadence tier:** targeted
- **Build:** `feat/stays-on-fix-b5`, behavior commit `9d766918a`; subsequent changes repair scoped lint without changing behavior.
- **Verdict:** PASS for the targeted product journeys.
- **Lab:** teardown completed at 2026-11-01T20:39:00Z with `clean: true`, zero survivors.

## Persona and charter

An operator maintains On demand and Global-only Stays on agents from All projects, without project
placements. They edit starting guidance, inspect Check-ins, and switch profiles to verify separate
persisted histories. Global definition and sidecar files remain shared; revision and snapshot
ownership is profile-specific. No provider turn is needed or launched.

| Journey | Result | Evidence under the manifest output directory |
|---|---|---|
| On demand What it does opens, saves and survives reload | PASS | `qa/ui-203113-on-demand-what-it-does-persisted.*`, `qa/ui-203126-on-demand-what-it-does-reloaded.*`; independent API reread |
| Global-only What it does opens, saves and survives reload | PASS | `qa/ui-203704-global-only-what-it-does-saved.*`, `qa/ui-203740-global-only-what-it-does-reloaded.*`; independent API reread |
| Global-only and On demand Check-ins open under All projects | PASS | `qa/ui-203217-global-only-check-ins-open.*`, `qa/ui-203518-missing-heartbeat-all-day-five-sources.*` |
| Missing HEARTBEAT.md displays all day and five sources | PASS | API `present=false`; UI 00:00 to 24:00; all five accessibility switches have `checked=true` in `ui-203518-*` |
| Default/editorial history isolation for both sidecars | PASS | timestamped API read/write/history receipts; distinct revision IDs; foreign rollback returns 404 |
| Missing default provider gives actionable copy and recovers | PASS | `qa/ui-203542-missing-default-provider-actionable-copy.*`, `qa/ui-203635-missing-provider-recovery-created.*`; Codex selection creates the agent with its original name, description and instructions |

Check-ins remain disabled while an agent has no project placement, preserving the existing UI
behavior; authoring read/write/history and rollback were exercised through the real public API.
The missing-file controls still display the complete defaults. The API independently confirmed
both UI-authored SOUL bodies and the newly created agent's provider/model.

## Verification and limits

Passed: `make codegen`, `go build -p 1 ./...`, focused owning race tests, `make codegen-check`,
Turbo Web typecheck and 137 tests across six affected Web suites. Full Soul and Heartbeat suites,
canonical migration integrity/apply/schema-equivalence tests, and native/extension heartbeat and
harness integration tests passed. COMPOZY_* was removed from Go check environments.

The optional complete globaldb suite exceeded its ten-minute package budget while extension,
automation and marketplace migrations were running. Focused B5 SQLite cases passed; no assertion
was weakened. Scoped lint findings in constants, parameter size, formatting and one fixture were repaired; the
final rerun passed with `0 issues`, exit code 0 (`term-09c88f47b9ab`). Final API/native/session
race checks passed.
Final build after those repairs passed. The parent report records the final lint receipt. Implementation and targeted QA are complete.

Strict evidence audit retains C14 because the worker brief explicitly prohibits the global local
gate it requests. The final verification report records this exception without claiming a gate
pass. No `make gate`, `make gate-full`, E2E, repository-wide lint/tests, real provider turn, push or
PR was performed. Targeted teardown stopped only verified lab-owned processes. The isolated
browser harness was stopped separately after its Chrome process exited.

## Session debrief

The first API leg stopped at a real @user resolver 404, fixed in production and reproduced by
the canonical API suite; later API and browser re-walks passed. A mistaken duplicate global
creation returned 409; the next leg selected the existing shared global definition. Early failing
receipts remain immutable. The initial `ui-203032-...-saved` label captures an unsaved draft and
is excluded from delivery proof; the later persisted/reloaded receipts are authoritative.
Onboarding was completed through the public CLI as lab setup, preserving the absent default
provider. Direct navigation initially showed Loading agent and was inspected again after it
settled. The integrated browser host and personal-Chrome permission were unavailable, so the
successful walk used a separate headless Chrome profile and CDP endpoint owned by this lab.

[QA_BOOTSTRAP]
manifest_path=/Users/pedronauck/dev/qa-labs/compozy-b5-profile-authoring-20261101-195941-993538-lab/qa-artifacts/qa/bootstrap-manifest.json
lab_root=/Users/pedronauck/dev/qa-labs/compozy-b5-profile-authoring-20261101-195941-993538-lab
runtime_home=/var/folders/7x/xg204hnd04b81fczcxvjlhzr0000gn/T/compozyqa-0510e6aeefc2/runtime
base_url=http://127.0.0.1:64948
verification_report=docs/qa/reports/2026-11-01-stays-on-b5-remediation.md
health_status=healthy
[/QA_BOOTSTRAP]
