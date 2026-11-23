# QA Run Report — 2026-11-23 — Skill lesson proposals (#1293)

- **Scope:** the remaining real-user journeys of compozy/compozy#1293 (spec `skill-lesson-proposals`, tasks 01–07) on the integrated head of PR #1968 (stacked on #1967, #1966, #1965): lesson review, edit review with stale base, revert, capped auto-approve, and install crash recovery with conflict resolution from two homes. Scenario: `MS-web-stays-on-reports`.
- **Cadence tier:** targeted.
- **Build:** `c6870831e` (`1293-s3q-qa`, PR #1968 head).
- **Status:** **PASS with one defect (D1)**. Walked 2026-11-23 (task_09). Details: `../scenarios/MS-web-stays-on-reports.md` "QA verification 2026-11-23 (#1293 task_09…".

## Personas

| Persona | Base | Device / Network / Locale | Sessions |
|---|---|---|---|
| Dora | Runtime Administrator | Linux desktop / wifi-fast / en-US | all five charters |

## Flows in Scope

- `J-31` — Steward an agent definition (`../journeys/J-31-steward-agent-definition.md`), extended to the always-on instance's skill reviews.

## Plan

Each journey runs in its own isolated lab from `eng-qa-bootstrap` (unique `COMPOZY_HOME`, port, socket), with every managed-session `COMPOZY_*` variable unset and only the lab's set, `[automation] enabled = false`, and a live Claude provider block. Journey 5 runs two homes on one project directory. Agent proposals go through the native tool `compozy__agent_skill_propose`, called by the always-on agent's session (a live turn, or `compozy tool invoke --session <main session>` when the walk needs exact inputs such as four eligible proposals in one UTC day); operator decisions go through the CLI, HTTP and the Web viewer.

| # | Charter | Entry path | Expected evidence |
|---|---|---|---|
| 1 | `CH-skill-lesson-review` | agent proposes a new skill with lesson and evidence → `agent skill review list/show` → Web card and viewer → `approve --expected-digest` | tool result `kind: new` with lesson and evidence; CLI Lesson block; card "When: <cue>"; viewer Lesson block with session links; skill active |
| 2 | `CH-skill-edit-review` | two `mode: edit` proposals from one base → `show` diff → Web Changes → approve the first → approve the second | `Base:`, counts, `Files:` and unified diff; "Change to review"; installed `SKILL.md` has the new step; second approve `skill_review_base_stale` (409), viewer disables Approve |
| 3 | `CH-skill-review-revert` | `show` `revert.allowed` → `revert --expected-digest` on the edit, repeat → revert of a superseded approval → Web Revert of a new skill | previous files restored (digest equals base); repeat succeeds unchanged; `skill_review_revert_conflict` `superseded`; "Reverted <name>." and skill gone from the catalog |
| 4 | `CH-skill-auto-approve` | toggle off proposal → `agent instance skill-auto-approve on` → 3 eligible + low-confidence + script proposals → 4th eligible → `review list --decided-via auto_approve` → Web card → revert → 5th eligible | `toggled_off`; on receipt; three `approved`/`auto_approve`; `low_confidence`, `runs_automatically`; fourth `daily_cap`; audit list; "Approved automatically"; revert does not refund the cap |
| 5 | `CH-skill-install-recovery` | freeze (SIGSTOP) the lab daemon at a journal phase during approve → SIGKILL → restart; external edit while frozen after `backup_staged` → restart → `install-conflict list|resolve` from home A and home B | rollback leaves the pre-approval tree and a pending review; post-commit kill finishes on restart; approve refused `skill_install_conflict`; viewer alert; both `--keep` values work, one from home B; list empty and approvals work afterwards |

## Reused slice evidence

- E2E-001 (journey 1, CLI and Web lesson review) walked on PR #1965 heads `2b103abb8` and `9ee1aa6e2`: `MS-web-stays-on-reports.md` "QA verification 2026-11-23". Journey 1 re-walks only what later slices changed on the same surfaces (the list columns and the card on the integrated head).
- Focused suites named in the scenario's QA impact notes own the states the walks do not reach (legacy "No lesson recorded", unavailable evidence session, rollback-table rows, digest counterexamples).
- Implementation reports `qa/task03-impl-report.md` … `qa/task07-impl-report.md` (workspace spec directory) list each slice's test owners.

## Regression hot spots

- `_spec.md` Safety Invariants 1–8: agent cannot decide or turn auto-approve on (journeys 1, 4); one transaction per skill across homes (5); single commit point, rollback before it and finalize after it (5); toggle and cap read under the instance mutex (4); rollback moves only journal-named entries and retains unexplained content as a conflict (5); external content never overwritten (2, 5); old tree removed only after commit (5); revert only when this review is the current writer (3).
- Registry package installs after the `internal/dirpublish` extraction (task_02): install and update one registry package (`compozy extensions install` / `update`) in a lab and confirm no `.compozy-*` staging leftovers remain.

## Session Matrix & Results

| # | Charter | Persona | Status | Evidence |
|---|---|---|---|---|
| 1 | `CH-skill-lesson-review` | Dora | Pass | [01](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/01-lesson-card.png) [02](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/02-lesson-viewer.png) [03](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/03-approved-receipt.png) |
| 2 | `CH-skill-edit-review` | Dora | Pass | [04](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/04-edit-cards.png) [05](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/05-edit-diff-viewer.png) [06](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/06-edit-base-stale.png) |
| 3 | `CH-skill-review-revert` | Dora | Pass | [07](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/07-revert-receipts.png) [08](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/08-reverted-new-skill.png) |
| 4 | `CH-skill-auto-approve` | Dora | Pass with D1 | [09](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/09-auto-approved-cards.png) [10](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/10-auto-approved-review.png); D1: agent refusal code `backend_unhealthy` instead of `skill_auto_approve_operator_only` |
| 5 | `CH-skill-install-recovery` | Dora | Pass | [11](https://raw.githubusercontent.com/jillesmc/compozy/pr-evidence/1293/pr3/c6870831e/11-install-conflict-alert.png); rollback, finalize, conflict from home B2, both `--keep` values |
| 6 | Registry install smoke | Dora | Pass | local install and `--replace` of a catalog package; no staging leftovers |

Status legend: `Pending | Pass | Fail | Blocked`

## Defects

- **D1** (journey 4, minor): a live agent turn calling `compozy__agent_instance_skill_auto_approve` with `{"enabled": true}` is refused, so Safety Invariant 1 holds, but the tool error code is `backend_unhealthy` and not `skill_auto_approve_operator_only` (`_dx.md` Native Tools). The message keeps the code text. Cause: `nativeInstanceError` in `internal/daemon/native_agent_instances.go` returns the agentinstance error without a domain `ToolError`. Reproduce: create an always-on instance, turn auto-approve off, then prompt its main session to call the tool with `{"enabled": true}`. The `tool_result` is `failed` with `backend_unhealthy: skill_auto_approve_operator_only: …`.

## Lab notes

- An early setup call ran without the lab variables and reached the operator's live daemon. It created workspace `platform`, agent `release-watch`, instance `ainst-3731d2f291590c5d` and main session `sess-c7503ed29f72c44a`; a proposal attempt failed schema validation, so no review was created. All four were removed through the live CLI within two minutes (instance archived, agent deleted, session stopped and archived, workspace removed). The lab wrapper now refuses to run without `LAB_HOME`/`LAB_PORT`.
- Fault injection used only SIGSTOP/SIGKILL of the lab daemon at an observed journal phase (`ptrace` is disabled on this host) and user-level edits of skill files; no journal, install log or SQLite was edited.
