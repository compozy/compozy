# loops-refac — QA cycle plan (planning report)

- **Scope:** the `loops-refac` workstream (TechSpec `.compozy/tasks/loops-refac/_techspec.md`, WS1–WS4). Consolidates the QA-impact flags from tasks 03/05/06/11/12/13/14 and plans the sessions task 16 (`qa-execution`) will walk.
- **Type:** **planning** (a `qa-report` deliverable, not a `qa-execution` run) — it consolidates flags into `state.csv`, mints the new `LP-040..LP-050` scenarios, resets the affected rows, and plans charters + the execution order. **No sessions were walked; every new/reset `LP-*` row is `qa_status: untested`.** Execution is task 16.
- **Cadence tier:** **Targeted** (branch/PR cycle scoped to the loops-refac diff) **with an e2e-web inclusion** — the diff carries UI-bearing changes (watch-events authoring form + run-detail park read-model), so the plan includes the browser lane a pure Targeted cycle would otherwise defer.
- **Author:** task 15 (`.compozy/tasks/loops-refac/task_15.md`).
- **Scope boundary (ADR-001):** the software-delivery **delegation rewrite is deferred** — no scenario tests native-task delegation, budget correlation (D3), approval placement (D4), or markdown authority (D5). The `execute_task` run-agent path stays; the only loop.yaml change in scope is the WS2 `load_tasks` node swap. (The `target_kind=loop` automation rows TA-063/064/073 + LP-034 are a *different, shipped* feature — automations delegating **to** loops — and are not this cycle's concern.)

This report is the single index task 16 reads to run the cycle: the persona roster, the scenario inventory, the risk-ordered charter roster, the execution order, the lab requirements, the evidence contract, the e2e-web scope, the flags↔tracker completeness audit, the coverage-taxonomy sweep, and the completeness validation.

---

## 1. Persona roster (`docs/qa/personas.md`)

| Persona | Base | Reality | This cycle's role |
|---|---|---|---|
| Bruno | Power User | desktop, keyboard, ships daily | Primary — operator authoring/parking/waking watch loops + verifying gated software-delivery sessions (web + CLI) |
| Ada | Power User (native-tool) | ACP agent, non-human actor | Primary — epic-watchdog watch loop + import-tool parity, entirely through structured surfaces |
| Marina | Casual User | phone-large, 4G, evaluator | Not exercised this cycle (no new approval/mobile surface); recorded skip |
| Lea | New User | laptop, evaluating vs Compozy | Not exercised this cycle (no onboarding-path change); recorded skip |
| Sol | Accessibility-Reliant | keyboard + screen reader | Deliberate partial — the watch-events editor form (LP-043) inherits the shared loop-editor a11y (CH-007/CH-013 own the canonical a11y walks); no net-new a11y surface, so no dedicated Sol charter this cycle (recorded skip, re-evaluate if the subscription form ships bespoke controls) |

Two primary personas (operator + agent) cover the two required perspectives the task names: **operator via web** (Bruno) and **agent via CLI/tools** (Ada).

---

## 2. Scenario inventory

### 2a. New `untested` rows (LP-040..LP-050) — flags consolidated from the implementation tasks

| Id | Scenario | Persona | Journey | Charter | Flag source |
|---|---|---|---|---|---|
| LP-040 | Park a watch-events loop and wake it on a matching task event | Bruno | J-16 | CH-022 | task 11 |
| LP-041 | Recover a parked watch-events loop after a daemon restart (replay) | Bruno | J-16 | CH-023 | task 11 |
| LP-042 | Watch a parent task child tree with an epic-watchdog loop | Ada | J-16 | CH-024 | task 11 (+ task 07) |
| LP-043 | Author a watch-events node in the loop editor | Bruno | J-16 | CH-022 | task 12 |
| LP-044 | Inspect the parked watch-events read-model on run-detail | Bruno | J-16 | CH-022 | task 12 |
| LP-045 | Import tasks through `ext__dev_cycle__import_tasks` with surface parity | Ada | J-07 | CH-027 | task 4/5 |
| LP-046 | Run a gated loop run-agent session with `allowed_tools` narrowing | Bruno | J-01 | CH-026 | task 3 |
| LP-047 | Wake a loop on an automation run completion (phase B) | Bruno | J-16 | CH-025 | task 13 |
| LP-048 | Wake a loop on a network message (phase B) | Bruno | J-16 | CH-025 | task 13 |
| LP-049 | Wake a loop on a coordinator lifecycle event (phase C) | Bruno | J-16 | CH-025 | task 14 |
| LP-050 | Wake a loop on a session event with content redaction (phase C) | Bruno | J-16 | CH-025 | task 14 |

### 2b. Reset rows (changed behavior → `untested`)

| Id | Scenario | Journey | Charter | Reason |
|---|---|---|---|---|
| LP-003 | Run software-delivery to a truthful verified done | J-01 | CH-026 | `load_tasks` action-node swap (task 5) + gated run-agent sessions (task 3) |
| LP-029 | Run reviews-watch through wake, remediate, and done | J-08 | CH-005 | `fix_batch` run-agent sessions now gated (task 3) |

Both reset rows were already `untested` from the incomplete prior cycle and carried task-03 flag notes; task 15 confirms the reset and appends the consolidation note (walking charter + overlap). **Not reset:** the reviews-watch terminal-classification rows LP-030/031/038/039 (`pass`) — those paths never reach a `fix_batch` session (clean-tick no-op, silent→stalled, blocked, gh-availability error), so gating does not change their observable. Flagging them would be over-flagging; they stay `pass`.

---

## 3. Charter roster (risk-ordered, `docs/qa/charters/`)

Ordered highest-blast-radius first — durability and redaction (silent-failure classes) ahead of happy-path authoring. Each binds exactly one persona + one tour + one journey.

| # | Journey | Persona | Tour | Box | Settles | Focus |
|---|---|---|---|---|---|---|
| CH-023 | J-16 | Bruno | Interrupt | 90 | LP-041 | Restart durability: wake exactly once from the ledger; backstop never claims |
| CH-025 | J-16 | Bruno | Feature | 90 | LP-047, LP-048, LP-049, LP-050 | Phase B/C family wakes + **session-record redaction non-leak** |
| CH-026 | J-01 | Bruno | Feature | 60 | LP-003, LP-046 | Gated software-delivery sessions + `allowed_tools` narrowing/widening; ext import consumption |
| CH-022 | J-16 | Bruno | Feature | 90 | LP-040, LP-043, LP-044 | Author → park (zero-cost, truthful read-model) → wake; non-match/cross-workspace no-wake |
| CH-024 | J-16 | Ada | Feature | 60 | LP-042 | Epic-watchdog watch loop, structured-surface parity |
| CH-027 | J-07 | Ada | Feature | 60 | LP-045 | Import-tool parity + deterministic ErrValidation across CLI/HTTP/UDS/native |
| CH-005 | J-08 | Bruno | Interrupt | 90 | LP-029 (+ LP-030/031/032/038 re-walk) | Reviews-watch under gating (fix_batch sessions gated) — durable charter re-run |

Every in-scope journey has ≥1 charter with an assigned persona: **J-16** (CH-022/023/024/025), **J-01** (CH-026), **J-07** (CH-027), **J-08** (CH-005). CH-005 is a durable charter re-run with a fresh debrief, not a new file.

---

## 4. Execution order for task 16

Run in this order — highest-risk/silent-failure classes first, dependencies respected:

1. **Bootstrap** a fresh isolated lab (§5) and persist the QA bootstrap block. Verify `make verify` green baseline on the branch before scenario execution.
2. **CH-026 (gated software-delivery, LP-003/LP-046)** — foundational: proves the run substrate is gated before watch scenarios layer on top; also settles the WS2/WS3 execution row.
3. **CH-027 (import parity, LP-045)** — agent-facing tool contract; cheap, unblocks confidence in the load_tasks payload CH-026 relies on.
4. **CH-022 (author → park → wake, LP-040/043/044)** — phase-A watch-events happy path; establishes the park/wake baseline the rest build on. Includes the **e2e-web** authoring-form + run-detail-park walks (§7).
5. **CH-023 (restart replay, LP-041)** — durability under a real lab daemon restart; run after the happy path is confirmed so a failure is unambiguously a recovery bug.
6. **CH-024 (epic-watchdog agent, LP-042)** — structured-surface parity on the phase-A mechanism.
7. **CH-025 (phase B/C families, LP-047..050)** — **gated on tasks 13/14 landing.** For any phase not shipped at run time, record the row `blocked-verify`/skip with reasoning (do NOT invent a pass — see §9). The redaction check (LP-050) is the security-critical assertion; inspect every loop-facing surface, not just the pill.
8. **CH-005 re-run (reviews-watch under gating, LP-029)** — durable charter, fresh debrief; confirms the fix_batch gating posture.
9. **Fix-loop** for any bug (red-before/green-after regression at the owning layer, `systematic-debugging` + `no-workarounds`), then the **full `make verify`** once as the completion gate.

---

## 5. Lab requirements (deterministic QA bootstrap)

Task 16 MUST NOT reuse a prior lab — a fresh, isolated pass:

- **Bootstrap:** `agh-qa-bootstrap` skill; a **fresh lab per pass** (do not reuse an older loops lab). Persist the machine-readable QA bootstrap block: `bootstrap-manifest.json` path, lab root, runtime home, base URL, and verification evidence.
- **Worktree isolation (mandatory — concurrency signaled):** unique `AGH_HOME`, unique daemon ports, unique `tmux-bridge` socket per worktree; the default home/port is forbidden. Activate `agh-worktree-isolation`.
- **Provider-home policy:** matches the provider contract — bound-secret/brokered creds use `PROVIDER_HOME`/`PROVIDER_CODEX_HOME` from the manifest; `native_cli` + `home_policy = operator` preserves the operator `HOME`/native login unless a scenario tests isolated provider-home.
- **Isolated web QA:** export `AGH_WEB_API_PROXY_TARGET` **derived from the bootstrap manifest/env** — never hardcode `:2123`.
- **Config writes:** `agh config set` and peers run **sequentially** per provider/runtime home — never parallelize config writes against one isolated home.
- **Watch-events durability scenarios (CH-023, LP-041)** require killing and restarting the lab daemon with the same `AGH_HOME` so the parked run and ledger persist across the restart; capture cursor/`last_wake_at` before and after.

---

## 6. Evidence contract

Every scenario verdict carries machine-checkable evidence paths recorded in `state.csv` (`evidence` column) and the dated report; lab scratch stays in the lab, indexed by path.

- **Watch-events wake (LP-040/047/048/049/050):** the `observe` events (matched/wake_enqueued) + the coordinator run rows proving the wake fired on the commit; the non-match/cross-workspace negative (no wake row).
- **Restart replay (LP-041):** cursor/`seq` comparison pre- vs post-restart proving replay-from-cursor and exactly-once (no double-wake row).
- **Gating (LP-046):** the session's sandbox/permission posture + narrowed tool set, and the deterministic widening **bind-error payload** for an out-of-profile tool.
- **Import parity (LP-045):** the tool-call record + the load_tasks node output showing byte-identical graphs; the deterministic ErrValidation message across surfaces.
- **Park read-model (LP-044):** `agh loop runs show -o json` showing subscriptions/cursors/last_wake_at, and the absent-block negative (a non-watch run renders nothing).
- **Redaction (LP-050):** the loop-facing event/output_ref + SSE + web run-detail proving record **content is absent** (only record_type/sequence/session correlation present).
- **UI (LP-043/044):** Playwright artifacts (§7) + `agh-ui-screenshot` captures cited by task 12.

---

## 7. e2e-web scope (UI-bearing changes present)

Task 16 MUST include the browser lane for the UI-bearing changes — **Playwright, no `force: true` actionability overrides**:

- **LP-043 (authoring form):** drive the loop editor → add a "Watch events" source node → assert the kind select renders only registry-supported kinds and the CEL filter per entry; a too-broad/unsupported entry gates Publish through the shared linter.
- **LP-044 (run-detail park state):** drive a parked run's detail → assert subscriptions/cursors/last_wake_at render; a non-watch run renders nothing (truthful UI).

Live-daemon browser E2E for these rides **AB-009** (the watch-events real-daemon seed harness) — the same gap class as AB-001. Until that seed lands, task 16 covers LP-043/044 at task-12's codec/component fixtures + `agh-ui-screenshot` and records the live-Playwright walk as AB-009-blocked (not a silent pass).

---

## 8. Flags ↔ tracker completeness audit (requirement 5 — no orphan flags)

Every implementation task's QA-impact line maps to ≥1 tracker row:

| Task | QA-impact line (verbatim intent) | Tracker rows | Status |
|---|---|---|---|
| task 03 | reset software-delivery + reviews-watch rows (sessions now gated); new gated-session scenario | LP-003 (reset), LP-029 (reset), **LP-046** (new) | ✅ landed |
| task 05 | reset software-delivery rows (`load_tasks` node shape changed) | LP-003 (reset), **LP-045** (import parity, overlaps) | ✅ landed |
| task 06 | covered by the task-05 flag — no additional rows | (folds into LP-003 / LP-045; web md_tasks removal = same behavior) | ✅ no orphan (explicit no-new-row) |
| task 11 | watch-events reachable end-to-end — new `untested` rows | **LP-040, LP-041, LP-042** | ✅ landed |
| task 12 | web authoring of watch-events + run-detail park state — new `untested` rows | **LP-043, LP-044** | ✅ landed |
| task 13 | automation-triggered + network-message watch scenarios | **LP-047, LP-048** | ✅ landed |
| task 14 | coordinator-watch + session-event-watch scenarios | **LP-049, LP-050** | ✅ landed |
| TechSpec §QA extras | epic-watchdog reference / import-tool parity / gated loop-session | LP-042 / LP-045 / LP-046 | ✅ landed |

**No orphan flags.** Tasks 01/02/07/08/09/10 carry **no** QA-impact rows (kernel neutralization, session gate, hooks catalog, DSL kind lint, store chokepoints, coordinator evaluator — all verified by `make verify`/unit lanes, surfaced downstream by task 11/12); their "no QA rows" declarations are intact.

---

## 9. Phase-gating & scope guards for task 16

- **Phase B/C rows (LP-047..050) depend on tasks 13/14 landing.** At plan time (2026-07-08) the watch-events execution chain (tasks 8/10/11/13/14) is pending per the workflow memory open-risk. Task 16 walks the **phase-A** rows (LP-040..044) and the WS2/WS3 rows (LP-003/029/045/046) first; for any phase not shipped at run time it records the row as `blocked-verify`/`skipped` with reasoning — **never a fabricated pass** (a skipped dimension recorded honestly beats a padded verdict).
- **No delegation scenarios (ADR-001):** none of LP-040..050 tests native-task delegation, D3 budget correlation, D4 approval placement, or D5 markdown authority. Verified by grep — the only "delegated" tracker rows are the shipped `target_kind=loop` automation feature.
- **Watch-events ≠ extension watch_source:** J-16 (daemon-internal, ADR-003, event-edged, no silence-stall) is distinct from J-08 (ADR-016 extension watch_source, poll/push, silence→stalled). Scenarios must not conflate the two source classes.

---

## 10. Coverage taxonomy sweep (five dimensions)

1. **Journeys** — J-16 mapped end-to-end (author→park→wake→recover) with abandonment paths (no-event dormancy, restart mid-park); J-01/J-07/J-08 reuse existing maps refreshed for the gating/import behavior. ✅
2. **Functional** — linter gates on unsupported kind/too-broad filter (LP-043), subset-only allowed_tools + widening rejection (LP-046), deterministic ErrValidation parity (LP-045), CEL match/non-match (LP-040/047/048). ✅
3. **Experiential** — truthful park read-model (dormant ≠ running ≠ terminal; absent renders nothing, LP-044); event-edged wake with no fake "checking…" state. ✅
4. **Edge / error / empty** — cross-workspace no-wake (LP-040), restart replay exactly-once (LP-041), redaction non-leak (LP-050), unsupported/pre-state kinds lint (LP-047..050), empty/malformed parse hard error (LP-045). ✅
5. **Cross-cutting** — workspace isolation at the doorbell (invariant 7, every wake row), agent↔operator parity (LP-042/044/045 structured surfaces), security redaction (LP-050). Mobile/a11y are **recorded skips** this cycle (no new mobile/a11y surface; the editor canvas is desktop-only per `personas.md`). ✅ (skips recorded)

No dimension silently skipped.

---

## 11. Cycle completeness validation (Step 7)

- [x] Every journey in scope has a flowchart with ≥1 abandonment path — J-16 authored (2 abandonment paths); J-01/J-07/J-08 pre-existing, refreshed.
- [x] Every journey in scope has ≥1 charter with an assigned persona — J-16 (CH-022/023/024/025), J-01 (CH-026), J-07 (CH-027), J-08 (CH-005).
- [x] Every new/reset scenario row has a stable `LP-NNN` id, a linked journey, and `qa_status: untested` — LP-040..050 new; LP-003/LP-029 reset.
- [x] Every QA-impact line from tasks 03/05/06/11/12/13/14 maps to ≥1 tracker row — §8 audit, no orphans.
- [x] No scenario references deferred delegation behavior — §9 scope guard (grep-verified).
- [x] The five taxonomy dimensions were considered; mobile/a11y skips recorded with reasoning (§10).
- [x] `state.csv` parses (16 fields/row) after the edits — verified.
- [x] No open bugs to register this cycle (planning only) — `bugs/` untouched; task 16 files any findings with dedup.

**Ready for `qa-execution` (task 16).** All planning artifacts are present and consistent in the living `docs/qa/` tree — no per-round `qa/` directory created; ids never reset.
