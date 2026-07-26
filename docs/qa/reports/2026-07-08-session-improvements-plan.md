# Session Improvements — QA cycle plan (planning report)

- **Scope:** the Session Experience Overhaul program (`.compozy/tasks/session-improvements/`, tasks 01–41) — the session render pipeline (blank-on-return, source flips, SSE reconnect), session open latency, the transcript UI language, and backend streaming.
- **Type:** **planning** (a `qa-report` deliverable, not a `qa-execution` run). It updates personas, maps journeys J-11…J-15, mints/links `RT-NNN` scenarios, and plans charters CH-014…CH-021. No sessions were walked — every session `RT-*` row is `qa_status: untested`. Execution is **task 43** (`qa-execution` + `agh-qa-bootstrap`).
- **Cadence tier:** **Full** (program gate, release-candidate class change per SD-005 — `make verify` is necessary but not sufficient for cross-component render drift).
- **Author:** task 42 (`.compozy/tasks/session-improvements/task_42.md`).
- **Consumes:** the pre-staged `_qa.md` base (§1–§9); `_techspec.md` (RC-1..11); `_tests.md` (automated lanes §6–§9); `analysis/summary.md` (root-cause chains); task 40 (telemetry) + task 41 (test-gap reproductions).

This report is the single index a reviewer or `qa-execution` reads: the persona roster, the journey inventory, the placeholder→final id mint ledger, the journey→`_tests.md` E2E + telemetry map, the taxonomy sweep, the risk-ordered charter roster, the E2E follow-up flags, and the completeness validation.

---

## 1. Persona roster (`docs/qa/personas.md`)

Session-experience personas (added / extended this cycle) alongside the existing Loops roster:

| Persona | Base | Reality | Owns (session) |
|---|---|---|---|
| **Théo** (new) | Power User | desktop, keyboard, long-lived background sessions | J-11 (hero), J-13 |
| **Nia** (new) | New User | laptop, judges AGH in 10 s | J-12, J-11 adjacents (canary) |
| **Rafa** (new) | Casual User | desktop, audits long transcripts | J-14 |
| **Ada** (extended) | Power User (native-tool) | ACP agent, non-human actor | **J-15** (+ J-07 Loops) |
| **Sol** (extended) | Accessibility-Reliant | keyboard + screen reader | J-13 a11y lens (CH-020) |

The `_qa.md` §2 "Automation Agent (CLI/API)" role maps to the existing **Ada** (extended, not duplicated). Mobile stays a device lens on Marina (glancing at a running session between meetings); accessibility is first-class (Sol) and extends over the redesigned thread. Five personas across the cycle — above the "≥3 personas per full cycle" bar.

---

## 2. Journey inventory (`docs/qa/journeys/`)

Five journeys cover the program's user-visible surface (`_qa.md` §3, placeholder J-A…J-E minted to J-11…J-15):

| Journey | Name | `_qa.md` | Personas | Abandonment path |
|---|---|---|---|---|
| **J-11** | Return to a running session (blank-on-return HERO) | J-A | Théo, Ada | cold-return closes tab mid-recovery; 5xx read as data loss |
| **J-12** | Open a session fast | J-B | Nia, Rafa | open stalls in first 10 s; unknown-id deep link |
| **J-13** | Follow a live run | J-C | Théo, Rafa | backgrounds tab mid-run; queues then leaves |
| **J-14** | Read a finished transcript | J-D | Rafa, Nia | output not inline-inspectable; paging skips/dupes |
| **J-15** | Operate a session via CLI/API | J-E | Ada | client killed mid-stream; read races a stop |

Every journey file carries a Mermaid flow with branch points + side effects + true end state + ≥1 abandonment path (qa-report journey rules).

---

## 3. Scenario map (`docs/qa/state.csv`, area `RT`)

### 3a. Existing rows linked in place (already `untested` — reset by their owning W1–W5 task; task 42 adds the journey link)

Per the flag-don't-retest rule, the tasks that changed these rows already reset `qa_status` to `untested`. Task 42 only adds the `journey` column (no status change, no duplication):

| Row | Journey | Owning tasks |
|---|---|---|
| RT-012 Get session snapshot | J-12 | 05, 12, 15, 22 |
| RT-013 Stop session | J-13 | 12, 21 |
| RT-015 Attach / resume session | J-11 | 01–09, 17 |
| RT-017 Clear conversation | J-14 | 08, 17 |
| RT-018 Send prompt (stream) | J-13 | 04, 16, 30, 39 |
| RT-019 Prompt while busy (queue/interrupt/steer) | J-13 | 35 |
| RT-020 Cancel prompt / cancel queued | J-13 | 35 |
| RT-022 Event/history/transcript/recap reads | J-14 | 14, 15, 24 |
| RT-023 Live session SSE stream | J-11 | 02, 04, 16, 17, 20, 44 |
| RT-024 Session health/status/inspect | J-11 | 22, 37 |
| RT-040 Session permalink redirect | J-12 | 10 |
| RT-041 Active workspace scoping (web) | J-11 | 13 |
| RT-043 Session thread transcript states | J-11 | 03 |
| RT-044 Session warm-return cache policy | J-12 | 06 |

RT-010 (create) and RT-011 (list) stay `pass` — their contracts are unchanged; the open path merely stopped traversing the full session list. Not reset (no behavior change).

### 3b. Mint ledger — placeholder ids finalized to `RT-NNN`

The individual UI/lifecycle tasks (21, 22, 25–37) appended granular scenario rows using the `_qa.md` placeholder ids (`NEW-6`, `NEW-7`, `NEW-12`…`NEW-22`, journeys `J-C`/`J-D`). Task 42 mints them to final `RT-NNN` ids, assigns the session persona + final journey, remaps cross-references in `overlaps`, and repairs the pre-existing unquoted-comma defect in `NEW-6`'s `entry_points`:

| Placeholder | Final | Journey | Persona | Title | Owning task |
|---|---|---|---|---|---|
| NEW-6 | RT-048 | J-14 | Rafa | Inline tool work grouping | 25/26/27 |
| NEW-7 | RT-049 | J-14 | Rafa | Session turn folding | 28 |
| NEW-12 | RT-050 | J-15 | Ada | Reads during stop and finalize | 21 |
| NEW-13 | RT-051 | J-15 | Ada | List/detail lifecycle convergence | 22 |
| NEW-14 | RT-052 | J-14 | Rafa | Inspector Usage tab shows real data | 37 |
| NEW-15 | RT-053 | J-14 | Rafa | Message hover toolbar (copy + timestamp) | 29 |
| NEW-16 | RT-054 | J-13 | Théo | Streaming indicators (typing dots + timer) | 30 |
| NEW-17 | RT-055 | J-14 | Rafa | Flattened grouped reasoning | 31 |
| NEW-18 | RT-056 | J-14 | Rafa | Per-tool icons + tense-aware verbs | 32 |
| NEW-19 | RT-057 | J-14 | Rafa | Unified tool status/error row states | 33 |
| NEW-20 | RT-058 | J-13 | Théo | Live-follow scroll modes + pill | 34 |
| NEW-21 | RT-059 | J-13 | Théo | Composer running semantics + queued rows | 35 |
| NEW-22 | RT-060 | J-14 | Rafa | Per-turn changed-files roll-up | 36 |

### 3c. Net-new rows minted (no task owned these hero/data-layer scenarios — they are the QA-tail's own)

| Row | Journey | Persona | Absorbs (`_qa.md` §4b) |
|---|---|---|---|
| **RT-045** Return to running session shows transcript | J-11 | Théo | NEW-1 (+ NEW-2 self-heal, NEW-10 return-badge, NEW-11 workspace-context) |
| **RT-046** Session opens in one loading phase | J-12 | Nia | NEW-4 |
| **RT-047** Long transcript is tail-first and paged | J-14 | Rafa | NEW-5 |

`_qa.md` NEW-3 (idle snapshot resubscribe) folds into RT-023 (already owns snapshot-on-subscribe). Session `RT` scenarios now run RT-001…RT-060; the transcript-UI-language cluster (RT-048…RT-060) all sits under J-14 by design — it is one cohesive audit journey, not two (the `~10-scenario` split heuristic is consciously not applied; the two J-14 charters CH-017/CH-021 split the walk instead).

---

## 4. Journey → `_tests.md` E2E lane + telemetry map (task 42 requirement)

`_tests.md` numbers its lanes by section: **§6** E2E-runtime, **§7** E2E-web, **§8** `agh-ui-screenshot` visual, **§9** manual QA lane. Telemetry is task 40.

| Journey | E2E-runtime (§6) | E2E-web (§7) | Visual (§8) | Manual (§9) | Telemetry (task 40) | Follow-up |
|---|---|---|---|---|---|---|
| **J-11** | 3 (snapshot-on-subscribe idle reconnect), 4 (list/detail consistency) | 1 (return hero), 6 (stop badge flip) | 1 (thread states) | 2 (hero), 3 (daemon-restart convergence), 5 (middle-gap reconnect) | empty-while-active; fetch-failure; SSE open/close/reconnect+cursor; gap-recovery; daemon active-stream/catch-up/assembly | **AB-005** (network-drop→reconnect Playwright) |
| **J-12** | 1 (long-session read snapshot cache) | 3 (cold open no reflash), 5 (deep link single spinner), 7 (tail-first + scroll-up) | 1 (thread states) | 7 (scroll-up lazy-load) | transcript assembly duration; catch-up batch size | **AB-008** (open-fast latency budget) |
| **J-13** | 2 (broadcaster gap-free stream), 4 (list/detail) | 2 (1k-event progressive), 4 (clear + reload), 8 (scroll hold), 9 (queue order) | 6 (working + reduced-motion) | 4 (clear-convergence) | active stream count; catch-up size; SSE lifecycle | **AB-006** (reduced-motion + streaming E2E sweep) |
| **J-14** | 1 (read cost) | 4 (clear persists), 7 (tail-first + paging) | 2–9 (tool-call language), 11 (changed-files) | 7 (paging) | read-path assembly duration (via J-12) | **AB-007** (grouping/fold pure-logic + axe a11y pass) |
| **J-15** | 1 (latency), 3 (snapshot seed), 4 (lifecycle consistency) | — | — | 6 (raw-stream contiguity `agh session events --follow`) | daemon slog: stream open/close, catch-up, assembly | **AB-008** (keep-alive proxy soak) |

Manual §9 items 3/4/5/6/7 are explicitly "owned by tasks 42/43" — they are pre-staged as charter must-tries (§9.3→CH-014, §9.4→CH-016, §9.5→CH-014, §9.6→CH-018, §9.7→CH-015/CH-021). Every journey has ≥1 automated backbone lane; each un-automated real-user branch has a charter, and each stable-but-unpinned flow has an AB entry.

---

## 5. Charter roster (`docs/qa/charters/`) — risk-ordered

| # | Journey | Persona | Tour / Mode | Box | Settles | Focus |
|---|---|---|---|---|---|---|
| **CH-014** | J-11 | Théo | Interrupt Tour | 60m | RT-045, RT-043, RT-023, RT-015, RT-024, RT-041 | blank-on-return HERO (background→return×3 + drop/restore + 5xx self-heal + badge truth) |
| **CH-018** | J-15 | Ada | Feature / strategy-based | 60m | RT-050, RT-051, RT-023, RT-022, RT-012, RT-042 | CLI+HTTP+UDS parity, snapshot resubscribe, stop-race, list/detail agreement |
| **CH-016** | J-13 | Théo | Multi-Tab Tour | 60m | RT-054, RT-058, RT-059, RT-018, RT-019, RT-020, RT-013 | follow a live run in two tabs; indicators, scroll follow, queue order, stop |
| **CH-017** | J-14 | Rafa | Feature Tour | 60m | RT-048, RT-049, RT-053, RT-055, RT-056, RT-057, RT-060 | tool-call visual language (grouping, inline I/O, icons, status, folds, copy) |
| **CH-021** | J-14 | Rafa | Garbage Tour | 60m | RT-047, RT-052, RT-022, RT-017 | transcript stress: paging gap-free, huge output, clear-persist, truthful usage |
| **CH-015** | J-12 | Nia | Network Tour | 30m | RT-046, RT-047, RT-040, RT-012, RT-044 | cold+deep-link opens of increasing size, one loading phase, no waterfall |
| **CH-020** | J-13 | Sol | Back-Button (a11y proxy) | 60m | RT-054, RT-058, RT-057, RT-048, RT-059 | a11y: SSE announced, status not color-only, reduced-motion, keyboard reach |
| **CH-019** | J-11 | Nia | Back-Button Tour | 30m | RT-010, RT-015, RT-021, RT-041 | regression canary of adjacent create/attach/approve/scoping |

Order = highest-impact journey × highest-blast-radius tour first (hero → agent parity → live follow → UI language → transcript stress → open-fast → a11y → canary).

---

## 6. Coverage taxonomy sweep (cover-or-skip per journey)

| Dimension | Where covered |
|---|---|
| **Journeys** | §2 — five flows, each with true end state + ≥1 abandonment path |
| **Functional** | rides inside scenarios: status+body on the agent API walk (CH-018), round-trips (clear persists, queue dispatch landing), console/log clean during walks |
| **Experiential** | skeleton/streaming feel (CH-014/CH-016), reduced-motion (CH-020), perceived performance on a throttled link (CH-015), copy in user language (CH-017), paper cuts logged in debriefs |
| **Edge / error / empty** | 5xx self-heal + reconnect gap + stopped-while-away (CH-014), true-empty session + clear + huge output (CH-021), unknown-id 404 (CH-015), stop-race reads + client-kill reconnect (CH-018) |
| **Cross-cutting** | responsiveness rides Marina's mobile lens as a must-try (desktop-primary surface); regression canary CH-019; icon/verb/status consistency CH-017; continuity = warm/cold return CH-014; a11y CH-020 |

**Deliberate skips (recorded, not silent):** cross-device continuity (product does not promise session hand-off phone↔laptop); locale/i18n sweep (session surface is English-only today); keep-alive proxy soak (environment-specific → AB-008, not a persona session); a dedicated mobile-session charter (mobile is a Marina device-lens must-try on CH-015/CH-017 per `personas.md`, the session surface being desktop-primary).

---

## 7. Completeness validation (qa-report Step 7)

1. **Every in-scope journey has a flowchart with an abandonment path** — J-11…J-15 ✔ (each Mermaid flow carries branch points, side effects, true end state, ≥1 abandon path).
2. **Every in-scope journey has ≥1 charter with an assigned persona** — J-11: CH-014 (Théo) + CH-019 (Nia); J-12: CH-015 (Nia); J-13: CH-016 (Théo) + CH-020 (Sol); J-14: CH-017 + CH-021 (Rafa); J-15: CH-018 (Ada). ✔
3. **Every in-scope scenario row has a stable id, a linked journey, and `qa_status` reflecting reality** — RT-045…RT-060 minted + 14 existing RT rows linked; all session rows `untested` (planning). ✔ (CSV validated: 335 rows, all 16 columns, no `NEW-*`/`J-A..E` placeholders remain.)
4. **Every open bug has a registry file + appears in a row's `bug_ids`** — no new bugs at plan time (execution files them, task 43); the blank-thread / false-`done` / fake-metric classes are pre-declared Blocks-Completion/Trust-Damage blockers (`_qa.md` §7).
5. **The five taxonomy dimensions were considered** — §6, with skips recorded.

**Hero-path truthful-UI observables (test-plan requirement):** RT-045 + CH-014 enumerate the explicit observables — never a false `ThreadEmpty` while persisted messages exist, never a false `done`/`running` badge, no silent permanent blank after a transient 5xx. RT-043 (states) and RT-023 (snapshot-on-subscribe) back them; task-40 counters make the empty-while-active decision point diagnosable.

---

## 8. Handoff to task 43 (`qa-execution` + `agh-qa-bootstrap`)

- **Lab fixtures the pass must provide** (`_qa.md` §8): a genuinely running background session (long turn in flight), a 1k+ event finished session, a failed session with a `failure` payload, an empty session, and a second workspace for the switch-notice branch.
- **Bug policy** (`_qa.md` §7): blank thread while persisted messages exist, any false lifecycle badge, empty-state copy during load/error, fake/permanently-empty metrics, and silent context-losing redirects are Blocks-Completion/Trust-Damage blockers; dedup against `BUG-0001..0019` before filing; every `state.csv` `fail` carries a `BUG-NNNN`; fixes only under the fix-loop governor.
- **Deterministic bootstrap:** fresh lab per pass; isolated `AGH_HOME`/ports/`tmux-bridge` sockets when concurrency is signaled; provider home-policy per L-016; export `AGH_WEB_API_PROXY_TARGET` from the manifest (never hardcode `:2123`); config writes against one home run sequentially.
- **Outputs:** lean evidence → `docs/qa/evidence/2026-07-08-session-improvements/` (checkpoints + failures only, skeeper-managed); dated run report → `docs/qa/reports/2026-07-08-session-improvements.md` (Final Status + release verdict with totals by user-impact tier); close with the machine-readable QA bootstrap block (manifest path, lab root, runtime home, base URL, report path).
