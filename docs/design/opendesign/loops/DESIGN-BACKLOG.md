# Loops design — status, review record, and Visual Contract approval

> Companion to `DESIGN-LESSONS.md` (the approved directive source). This file is the execution
> record for the `loop-node-lifecycle` Visual Contract: status, the closed review round, the
> locked decisions, and the approval that unblocks web Visual Contract tasks `task_08` (run UI),
> `task_09` (editor lifecycle grammar + chrome), and `task_10` (hero path). Delete it when
> task_08 + task_09 + task_10 have consumed the artboards.

## 1. Status snapshot — 2026-08-02 (closed)

| Surface | File | State |
| --- | --- | --- |
| Launcher / map | `index.html` | ✅ current (finals × labs, `_done` seam stated) |
| Loops catalog (final) | `loops-catalog.html` | ✅ hero-path promotion, directive-compliant (2026-08-02) |
| Run form (final) | `loop-run-form.html` | ✅ hero-path promotion, arrive-and-use (2026-08-02) |
| Loop page (final) | `loop-detail.html` | ✅ approved direction (directive-compliant) |
| Loop editor (final) | `loop-editor.html` | ✅ full lifecycle grammar (see §2.2 — closed) |
| Editor chrome states (lab) | `loop-editor-states.html` | ✅ read-only/fork + publish-rejected (2026-08-02) |
| Run page (final) | `loop-run-detail.html` | ✅ approved direction (canonical timeline, Lucide, collapse) |
| Run-page state matrix | `loop-run-detail-states.html` | ✅ reviewed — revise-with-notes, fixes applied (§2.1) |
| Node controls lab | `loop-node-controls.html` | ✅ reviewed — revise-with-notes, fixes applied (§2.1) |
| Quarantine sheet | `loop-quarantine-sheet.html` | ✅ reviewed — revise-with-notes, fixes applied (§2.1) |
| Inventories | `loop-inventories.html` | ✅ reviewed — revise-with-notes, fixes applied (§2.1) |
| Directives source | `DESIGN-LESSONS.md` | ✅ approved; §D promoted (§4.3 — closed) |
| Visual Contract approval | this file (§4) | ✅ recorded 2026-08-02 |

## 2. Review record

### 2.1 · Lab review round — closed 2026-08-02

Each lab was reviewed against the directive checklist (§3) + the one-story canon. All four came
back **revise-with-notes**; every finding was applied to the same files:

- **`loop-run-detail-states.html`** — 13 findings: gen-2 story contradiction in `state-quarantined`
  (task_02 event replaced with a task_04 lane event), newest-first ordering, ladder timestamp
  (`re-notified · 14:40`), inventory anchor retargets (`#agg-bell-*`), retry arithmetic aligned to
  the episode-1 ledger (attempt 2 next `14:33:41`, backoff 1m — an 8m backoff would exceed
  `retry.backoff max 5m`), parked-time math (22 lane-minutes), summary gists restored, authority
  comments added, accent budget (canceled state), control gates cited, fan-out item identity
  (`task_03[1]`), Needs-you re-anatomied to `panelbox`/`needsrow`, icon budget split
  (`redo-2` requeue · `volume-off` silence · `shield-alert` entry).
- **`loop-node-controls.html`** — 7 findings: **`Retry now` deleted** (resume is gated on paused —
  the page's own `node_not_paused` panel refutes it; skip-backoff documented as Pause… →
  Resume now), `aria-modal` on all 7 dialogs, `.btn--danger-solid` annotated as authorized delta,
  citations repointed to `_done/modals/modal-system.css` (`MODAL-STANDARD.md` does not exist),
  closing explainer note deleted, `mode: drain` machine truth in the pause dialog, RadioCard
  geometry restored (+28px icon wells).
- **`loop-quarantine-sheet.html`** — 10 findings: run clock `31m 12s` (sheet sits after episode 2),
  requeue confirm aligned to `_done/modals/modal-system.css` (blur, `--width-modal-sm`,
  `--radius-icon-well:10px`, 15px/13px type, body padding), `Cancel` verb renamed + gated,
  attempt-1 end time `14:32:41`, unused `.pillxs` deleted from the seed, `.tldiv` episode divider
  annotated, hint card re-chromed to quiet-note (danger stays the only tone), production path
  fixed to `run-page/loop-run-inspect-sheet.tsx`, backdrop gained Progress + Happening-now with
  task_03 parked (`2 of 3 active tasks`), dead link fixed.
- **`loop-inventories.html`** — 8 findings: Lucide state icons on rows/cards (hourglass ·
  shield-alert · triangle-alert · rotate-ccw), task_03 removed from `retrying` (post-ep-2
  snapshot), killed run split to `r-3c77e1` (r-914c3d stays the live triage run — catalog roster
  reconciled to `needs-approval · waiting on you · 49m`), approve-gate wait re-homed to
  `docs-refresh · r-2b81aa`, task_04 wait = `dependency — waits for task_03` (silence flag
  deleted; aggregates resynced 3→2 / 5→4), age-sort affordance removed (no `sort` param in the
  spec route), authority comments on agg-bell/agg-kpis, `history.replaceState` on the switcher.

One-story canon (all files agree): loop `software-delivery`, run `r-8f3a2b`, lanes
`task_01..task_04` with `task_03` troubled; 3 attempts per episode; episode 1 —
`transport 14:32:41 → attempt_timeout 14:38:44 → payload_declared 14:45:02`, quarantined
14:45:02; requeued by pedro 14:48; episode 2 quarantined 14:52:31; a further requeue opens
episode 3.

### 2.2 · Editor lifecycle grammar — closed 2026-08-02

`loop-editor.html` now shows the full grammar this pass chose to show, all tagged
`new · lifecycle` where proposal (authority chain per `DESIGN-LESSONS.md` L1/L3):

- Reliability envelope: `deadline`, `retry` (+ backoff + non_retryable), `result_contract`,
  `on_error` as the spec object — route XOR allow_fail select + its own effect list.
- The six plain node triggers (`on_retry` … `on_quarantine`) as structured effect lists —
  each entry exactly one of `emit`/`tool` (+ `with`), fail-open semantics stated at point of use.
- Contract tab: the seven terminal triggers (`on_done` … `on_canceled`) as effect lists,
  once-per-run (kill included) stated inline; DSL shows `on_failed` (tool) + `on_canceled` (emit).
- `wait` node on canvas + inspector (`await_deploy_ack`): `for | until | event` XOR select,
  `expect` schema, `ahead_arrival`, `expires` (+ escalate / route); the event subscription block
  is elided exactly as the TechSpec elides it.
- `on_parent_close` on a run-loop node (`release_notes`): `terminate | cancel | abandon`.
- Warning-severity lint row (`wait_expiry_without_path`) — visible in the dock, never gates
  Publish; error/warning counts split, zero counts render nothing.
- Read-only built-in source (fork-before-publish) + Publish-rejected strip live in the
  `loop-editor-states.html` companion (kept out of the final page to hold the line budget).

### 2.3 · `_done/loops` fate — decision closed (owner, 2026-08-02)

Option 1 executed: hero path (`loops-catalog.html`, `loop-run-form.html`) refreshed into
`loops/` as directive-compliant finals, cross-linked into the one-story set.
**`loop-configure.html` and runs history stay archived in `_done/loops/`** until their own specs
move; `index.html` states the seam explicitly. No silent redesign of those surfaces.

## 3. The method (kept for future passes)

Per surface: transcribe production first → seed chrome from the approved family final → apply
the directive checklist (Lucide-only · collapse anatomy · badge budget · canonical timeline ·
micro-mono machine truth · gated controls cited in comments · no explainer cards · deltas
annotated with authority) → self-check + cross-links → PNG evidence to `_captures/`.
This is now codified in `design-system/GUIDE.md` §Prototype directives and
`docs/_memory/lessons/L-034`.

## 4. Exit criteria — all met 2026-08-02

1. **Coverage matrices closed** ✅ — every TechSpec event kind, verb, and parked state maps to a
   designed treatment: 15 event kinds + verb routes across the labs (states matrix, node
   controls, quarantine sheet, inventories), lifecycle grammar in the editor, `canceled`
   terminal across catalog/run form/inventories/states.
2. **Approval recorded** ✅ — see §4.1.
3. **Directives promoted** ✅ — `DESIGN-LESSONS.md` §D → `design-system/GUIDE.md`
   §Prototype directives (hard rules) + `docs/_memory/lessons/L-034-prototype-production-transcription-first.md`
   (+ index row). The lessons file stays as the evidence record.
4. **README/index truthful** ✅ — `opendesign/README.md` loops row unchanged and correct;
   `index.html` reflects 5 finals × 5 labs and the `_done` seam.

## 5. Post-approval round — start-binding authoring (2026-08-02)

**Owner critique (initial):** start triggers were the one part of the definition with no authoring
surface — a read-only chip strip (`startstrip`) while every graph node is draggable and configurable.

**Owner revision (same day, post first draw):** surface kinds (`manual` · `cli` · `http` · `uds` ·
`native_tool`) are allowlist membership, not graph structure. Drawing all ten kinds as dashed
canvas nodes over-represented surfaces and cluttered the fan-in. Surfaces must leave the canvas /
palette and live as boolean controls in a dedicated **Start** inspector lane
(`Contract | Start | Node`). Contract stays goal / DoD / terminal reactions only. Hands-free kinds
keep richer authoring (allowlist + attached Job/Trigger). Canvas keeps a compact chip summary of
declared `start[]` kinds — not binding nodes.

**Research verdict (3 parallel deep-dives: spec · production · trigger-UI patterns):**

- The `loop-node-lifecycle` spec is **silent** on `start[]` — ingress policy is Spec 3 domain.
  Authority is shipped code/docs: `dsl.StartBinding{kind, inputs, input_mapping}` (gate_start.go),
  closed vocabulary of 10 kinds; **no per-kind config on the binding** — cron/endpoint/event live
  on the automation Job/Trigger that targets the loop (`dsl-reference.mdx` §Start bindings).
- Docs call the editor strip read-only, but `loop-editor-start-summary.tsx` itself names editable
  `start[]` "a v1 deferral", and `PATCH /loops` already round-trips `start[]`
  (`LoopDefinitionDocument.Start`) — the proposal closes a deferral, no new verb needed.
- Truths surfaced into the design: web Run form starts over `http` (`manual`/`cli` have no wired
  producer today); `native_tool` = `compozy__loop_run`; `schedule` rejects `input_mapping`
  (`start_input_mapping_invalid`); mapping values must be exactly `{{ .trigger.payload.<field> }}`;
  removal answers `start_kind_not_allowed`; no start-shape lint exists (nothing invented).
- `dsl.Contract` has no `start` field — start toggles must not live under Contract.

**Drawn in `loop-editor.html`** (tagged `new · start-authoring · owner revision`):
right rail `Contract | Start | Node`; Start lane = surface Switch allowlist (on/off ↔ presence in
`start[]`, optional static `inputs`, http-off warns that the web Run path dies) + hands-free rows
(allowlist + attached Job/Trigger folds — cron/endpoint/secret on the automation, not the binding);
palette keeps only `Start · hands-free` shortcuts (declare kind → open Start lane); canvas restores
the compact chip summary synced to the allowlist (no surface/hands-free binding nodes, no fan-in);
DSL `start:` block stays the single allowlist representation. `loop-run-form.html` links to the
Start inspector (and correctly stamps this form as `http`). **Ship gate:** Spec 3 adjacency —
editor writes `start[]`, production sidebar gains the Start lane, docs drop read-only strip
language; until then Visual Contract material only. Held outside Spec 1 per
`.compozy/tasks/loop-node-lifecycle` MVP Boundary / Non-Goals / ADR-018 — survives deletion of
this backlog file.

**Capture:** `_captures/loops-loop-editor.png` refresh **skipped** this pass — OD `export … --format image`
failed in-environment (`Unable to find helper app` / GPU process exit). Prior PNG remains stale
relative to the Start-lane revision; re-export when the helper is available.

### 4.1 · Approval record

**2026-08-02 — Visual Contract approved.** Recorded per the owner work-order
`docs/prompts/20260802-0107_loops-lifecycle-design-gaps.md` (Pedro, 2026-08-02), which defined
the approval conditions this file now evidences: lab review closed with fixes applied (§2.1),
editor grammar coverage closed (§2.2), hero-path promotion done (§2.3), directives promoted
(§4.3), PNG evidence in `_captures/loops-*.png` for every artboard listed in `index.html`
(the OD exporter is unblocked; the earlier repo-side-only note is obsolete).
**This unblocks web Visual Contract tasks `task_08` + `task_09` + `task_10`** — the artboards
under `docs/design/opendesign/loops/` + this record are the Visual Contract gate; ownership
split is run UI / editor / hero path per ADR-018; no external plan file is authoritative.
