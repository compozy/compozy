# Loops — QA cycle plan (planning report)

- **Scope:** the Loops feature (PRD `.compozy/tasks/loops/_prd.md`), full-feature QA plan for the first cycle.
- **Type:** **planning** (this is a `qa-report` deliverable, not a `qa-execution` run) — it maps journeys, mints `LP-NNN` scenarios, and plans charters. No sessions were walked; every `LP-*` row is `qa_status: untested`. Execution is task 25 (`qa-execution`).
- **Cadence tier:** **Full** (release-candidate posture — all P0/P1 journeys, every project persona covered).
- **Author:** task 24 (`.compozy/tasks/loops/task_24.md`).
- **Design reference:** `docs/design/opendesign/` (`LOOPS-DESIGN-SPEC.md` §4) — every UI-bearing journey/charter cites its screen.

This report is the single index a reviewer or `qa-execution` reads to see the whole plan: the persona roster, the journey inventory, the journey→`_tests.md` E2E backbone matrix, the coverage-taxonomy sweep, the risk-ordered charter roster, the E2E follow-up flags, and the completeness validation.

---

## 1. Persona roster (`docs/qa/personas.md`)

| Persona | Base | Reality | Owns |
|---|---|---|---|
| Bruno | Power User | desktop, keyboard, ships daily | J-01, J-02, J-04, J-05, J-06, J-08, J-10 |
| Lea | New User | laptop, evaluating vs Compozy | J-01, J-02 |
| Marina | Casual User | phone-large, 4G, evaluator | J-03, J-08, J-09 |
| Ada | Power User (native-tool) | ACP agent, non-human actor | J-07 |
| Sol | Accessibility-Reliant | keyboard + screen reader | J-03, J-05 (a11y lens) |

Mobile is a device lens on Marina (read/approve surfaces); the loop editor canvas is desktop-only (recorded skip in `personas.md`). Accessibility is a first-class persona (Sol), not a skip. Five personas across the cycle — well above the "≥3 personas per full cycle" bar.

---

## 2. Journey inventory (`docs/qa/journeys/`)

The 7 journeys required by task 24, plus 3 that the full PRD experience demands (their absence would be a real coverage gap a reviewer would flag):

| Journey | Name | Required? | Personas | Design screen |
|---|---|---|---|---|
| J-01 | Arrive-and-use run (hero) | required | Lea, Bruno | loops-catalog §4.1, loop-run-form §4.3, run-detail §4.4 |
| J-02 | Dry-run preview | required | Lea, Bruno | loop-run-form §4.3 |
| J-03 | Observe and approve | required | Marina, Sol | runs §4.5, run-detail §4.4 |
| J-04 | Operator pause/resume/stop | required | Bruno | run-detail §4.4 |
| J-05 | Configure (no-fork) | required | Bruno, Sol | loop-configure §4.7 |
| J-06 | Fork and edit | required | Bruno | loop-editor §4.6 |
| J-07 | Agent-operated run | required | Ada | structured surface (no screen) |
| J-08 | Watch and maintain | added (use case #1, watch-source/extension) | Bruno, Marina | run-detail §4.4, loops-catalog §4.1 |
| J-09 | Automation start-bindings | added (§9.14 web surface) | Marina, Bruno | loop-detail §4.2 |
| J-10 | Converse and decide | added (use case #3, the differentiator) | Bruno | run-detail §4.4 |

Every journey file carries a Mermaid flowchart with branch points, side effects, the true end state, and ≥1 abandonment path, plus a YAML map and an `e2e_backbone` block. Design screen `loops-index.html` (§4.8) is intentionally uncovered — it is a design-doc launcher, not a product route.

---

## 3. Journey → `_tests.md` E2E backbone matrix

Each journey maps to concrete E2E-runtime / E2E-web cases (the executable backbone); integration/unit/component cases are listed in the journey files. `⚠` marks a flow that needs an E2E follow-up (automation-backlog entry).

| Journey | E2E-runtime | E2E-web | Follow-up |
|---|---|---|---|
| J-01 | R-1, R-7 | W-1, W-2, W-3, W-4, W-5 | ⚠ AB-001 (real-daemon seed harness) |
| J-02 | — | W-2 (dry-run) | ⚠ AB-001 |
| J-03 | R-8, R-10 | W-7, W-9, W-10 | ⚠ AB-001, AB-004 |
| J-04 | R-6 | W-8 | ⚠ AB-001 |
| J-05 | — | W-11 | effective-config GET gap (flagged in J-05 file) |
| J-06 | — | W-12, W-13, W-14, W-15, W-16 | ⚠ AB-001 |
| J-07 | R-3, R-5, R-8 | — | ⚠ AB-003 (parity harness) |
| J-08 | R-2, R-7, R-9 | W-9 | ⚠ AB-001 (watch seed), push-path deferred |
| J-09 | R-5 | W-17 | ⚠ AB-001 |
| J-10 | — | W-6 | ⚠ AB-002 (no installed template — needs a seed) |

Every journey maps to at least one concrete `_tests.md` case OR is explicitly flagged for follow-up. No journey is left unmapped.

---

## 4. Coverage taxonomy sweep (five dimensions)

Applied per the taxonomy reference — each dimension is either covered or a deliberate skip is recorded (no silent gaps):

1. **Journeys** — 10 mapped end-to-end with true end states + abandonment paths (§2). ✅
2. **Functional** — auto-form validation/gating (LP-002/007), override clamp (LP-004/018), CAS 409 (LP-024), allowlist 422 (LP-035), capability gate (LP-027) ride inside the journey scenarios. ✅
3. **Experiential** — perceived performance/first-impression (CH-001 Lea), truthful meters + terminal banner (CH-012), reduced-motion pulse (CH-011 `must_try`). ✅
4. **Edge / error / empty** — dry-run no-leak (LP-006/007), silent watch → stalled (LP-031), impossible dependency → blocked (LP-038), no-decision → stalled (LP-037), pause-preemption (LP-015), reject-halt non-done (LP-011), empty Custom catalog group (J-01 flow). ✅
5. **Cross-cutting** — accessibility (CH-011 across J-03+J-05), mobile approve (LP-013), agent↔operator parity (J-07/CH-004), workspace isolation of the loop-target automation (LP-035). Responsiveness at 375/768/1280 is a **deliberate partial**: covered for read/approve via Marina; the desktop-only editor canvas is a recorded skip (see `personas.md`). ✅ (skip recorded)

No taxonomy dimension is silently skipped.

---

## 5. Charter roster (risk-ordered, `docs/qa/charters/`)

Ordered highest-impact journey × highest-blast-radius first. Each binds exactly one persona + one tour + a time-box; every UI charter names a design screen and a truthful-UI check.

| # | Journey | Persona | Tour | Box | Focus |
|---|---|---|---|---|---|
| CH-001 | J-01 | Lea | Feature | 60 | Arrive-and-use ≤ Compozy (adoption risk) |
| CH-002 | J-03 | Marina (phone) | Interrupt | 60 | Truthful approval; needs-approval ≠ terminal |
| CH-003 | J-04 | Bruno | Interrupt | 60 | Pause truthful at boundary; Stop→failed |
| CH-004 | J-07 | Ada | Feature | 60 | Structured parity + no self-approval |
| CH-005 | J-08 | Bruno | Interrupt | 90 | watching zero-cost; no-op/stalled truthful |
| CH-006 | J-05 | Bruno | Back-Button | 60 | No-fork boundary; no cost cap |
| CH-007 | J-06 | Bruno | Multi-Tab | 90 | Linter authority; Publish gate; CAS 409 |
| CH-008 | J-02 | Lea | Garbage | 30 | Dry-run: no run/no budget leak |
| CH-009 | J-09 | Marina | Back-Button | 60 | start[] allowlist; binding badge |
| CH-010 | J-10 | Bruno | Feature | 60 | Channel harvest real; no-decision→stalled (blocked on AB-002) |
| CH-011 | J-03 | Sol | Back-Button | 60 | Run-status not color-only; approval-dialog focus trap; reduced-motion |
| CH-012 | J-01 | Bruno | Feature | 60 | Truthful run monitor + meters (regression canary) |
| CH-013 | J-05 | Sol | Back-Button | 60 | Configure-sheet focus trap; labelled controls; cancel writes nothing |

Every journey has ≥1 charter with an assigned persona (J-01: CH-001+CH-012; J-03: CH-002+CH-011; J-05: CH-006+CH-013). CH-011 (J-03 a11y) and CH-013 (J-05 a11y) each walk a single journey so the coverage ledger stays greppable.

---

## 6. E2E follow-up flags (automation backlog)

Recorded in `docs/qa/automation-backlog.md` — not as metadata on scenarios:

- **AB-001** — Loop web E2E seed harness (real-daemon Playwright): the daemon now emits rich Loop SSE frames, but `web/e2e/fixtures/*` still has no loop seed that drives those states in Playwright. Blocks real-browser E2E for J-01/03/04/06/08/09; covered meanwhile at daemon/runtime tests, vitest/component, and `agh-ui-screenshot`.
- **AB-002** — Converse-and-decide seed: no installed template (docs-only); J-10/CH-010 need a hand-built `agh__network_send` + `channel_result` seed to exercise E2E-web-6.
- **AB-003** — Agent-operability parity harness for the full `agh__loop_*` verb set (J-07).
- **AB-004** — Seeds that produce all 11 statuses (incl. `no-op`/`blocked`/`queued`/`paused`) to pin the no-coercion invariant (J-03/J-08).

**Runtime open risks carried in from prior tasks (not this cycle's bugs):** no effective-config GET endpoint (J-05 inherited-default placeholders can't be shown truthfully vs the operator `[loops.defaults.*]` layer); the run-page forward-contract gap (no pinned executed-definition/gate_id/started_at). Both are flagged for `qa-execution` to verify save-then-run / historical-run behavior empirically rather than trust the UI copy.

---

## 7. Cycle completeness validation (Step 7)

- [x] Every journey in scope has a flowchart with ≥1 abandonment path — 10/10.
- [x] Every journey in scope has ≥1 charter with an assigned persona — 10/10 (CH-001..CH-013).
- [x] Every scenario row has a stable `LP-NNN` id, a linked journey, and `qa_status: untested` (planning) — LP-001..LP-038.
- [x] Every UI-bearing charter/scenario cites a design screen + a truthful-UI check.
- [x] Each journey maps to a concrete `_tests.md` E2E case or is flagged for follow-up (§3).
- [x] The five taxonomy dimensions were considered; the one partial (responsiveness / desktop-only editor) is recorded with reasoning, not padded (§4).
- [x] No open bugs to register this cycle (planning only) — the `bugs/` registry is untouched; `qa-execution` files any findings with dedup.

**Ready for `qa-execution` (task 25).** All planning artifacts are present and consistent in the living `docs/qa/` tree — no per-round `qa/` directory was created.
