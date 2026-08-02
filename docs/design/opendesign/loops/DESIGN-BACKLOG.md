# Loops design — what's left and how to proceed

> Companion to `DESIGN-LESSONS.md` (the approved directive source). This is the execution map:
> current status, the remaining design work in priority order, and the method each item must
> follow. Update this file as items land; delete it when the set is fully approved and task_08
> consumes the artboards.

## 1. Status snapshot

| Surface | File | State |
| --- | --- | --- |
| Launcher / map | `index.html` | ✅ current |
| Loop page (final) | `loop-detail.html` | ✅ approved direction (directive-compliant) |
| Loop editor (final) | `loop-editor.html` | ✅ production-parity + spec deltas; states pending (see 2.2) |
| Run page (final) | `loop-run-detail.html` | ✅ approved direction (canonical timeline, Lucide, collapse) |
| Run-page state matrix | `loop-run-detail-states.html` | 🔎 re-patterned onto the approved run page (2026-08-02) — awaiting review |
| Node controls lab | `loop-node-controls.html` | 🔎 re-patterned (MODAL-STANDARD chrome, routes cited) — awaiting review |
| Quarantine sheet | `loop-quarantine-sheet.html` | 🔎 re-patterned (canonical timeline chain, real backdrop) — awaiting review |
| Inventories | `loop-inventories.html` | 🔎 re-patterned (Rows\|Cards contract, derived counts) — awaiting review |
| Directives source | `DESIGN-LESSONS.md` | ✅ approved; promotion pending (see 4.3) |
| Spec approval log | `.compozy/tasks/loop-node-lifecycle/_prototype.md` | ❌ still `- _pending_` |

The four ⚠️ labs are the spec's coverage-matrix carriers (every event kind, verb, and parked
state must have a designed treatment) — they cannot be dropped, only re-patterned.

## 2. Remaining design work (priority order)

### 2.1 · P1 — Re-pattern the four labs onto the approved base — ✅ executed 2026-08-02, awaiting review

Executed notes: all four labs now seed the `loop-run-detail.html` chrome (verbatim token/style
block), tell the one story (`software-delivery` / `r-8f3a2b`, task lanes `task_01..task_04` with
`task_03` as the troubled lane), and share a single reconciled failure arc — 3 attempts per
episode; episode 1 quarantined 14:45 (`transport → attempt_timeout → payload_declared`),
requeued by pedro 14:48, episode 2 quarantined 14:52, a further requeue opens episode 3. This
resolves the prior episode-number divergence across the labs. Coverage matrices preserved
(spot-check against §2.1 notes below). PNG captures still pending repo-side (`eng-ui-screenshot`).

All four follow the same recipe: seed the page chrome from the NEW `loop-run-detail.html`
(head, sections, timeline, rail), then apply the directive checklist (§3). Specific notes:

1. **`loop-run-detail-states.html`** — rebuild each of the 8 states as a variant of the new
   run page, not the old one. Per state, only the affected panels change; everything else
   stays identical to the final page (that is the point of a matrix). Tone map stays:
   retrying = warning · waiting/paused = info · quarantine = danger · attention = warning ·
   canceled = neutral. The `state-waiting` section must show the approval escalation ladder
   position (`re-notified → escalated`) and fan-out lane identity; `state-parked-progress`
   keeps parked lanes out of the denominator with the clock suspended and tokens counting.
2. **`loop-node-controls.html`** — menus/confirms/dialogs move onto `modals/MODAL-STANDARD.md`
   chrome with Lucide icons; every verb row cites its route in a comment (directive #11).
   The four deterministic answers (`node_not_paused`, `node_not_quarantined`, `run_terminal`,
   `already_decided{winner}`) render as quiet informative panels, not error toasts.
3. **`loop-quarantine-sheet.html`** — re-seat the 560px sheet over the NEW run page backdrop;
   production pattern is `loop-run-inspect-sheet.tsx`. Chain rows adopt the canonical timeline
   anatomy (22px tone-ring dots, micro mono `failure_class` + `disposition`); the remediation
   hint leads; truncation stays visibly marked.
4. **`loop-inventories.html`** — single surface with the 4-state switcher (mirrors
   `?state=`); listing toolbar keeps the Rows|Cards contract; rows get Lucide state icons and
   the badge budget (state pill only, reason as text, age mono). Include the truthful empty
   state per filter and the runs-area `canceled` deltas (outcome filter, pill, KPI).

### 2.2 · P2 — Editor states the final page doesn't show

Add to `loop-editor.html` (or a small `-states` companion if it would exceed ~1000 lines):

- **Wait node on canvas + inspector** — the palette ships the `wait` kind but no node uses it.
  Add a canvas demo (or a second demo loop) showing `for | until | event` (XOR), `expect`,
  `ahead_arrival`, and the `expires` group; inspector fields per the TechSpec wait block.
- **`on_parent_close` on a run-loop node** — the one envelope field still without a designed
  surface (`terminate | cancel | abandon`, run-loop only).
- **Warning-severity lint row** — the dock only demos a blocking error; add one warning
  (`wait_expiry_without_path` or the agent-retry large-cap warning) showing that warnings
  don't block Publish.
- **Read-only source state** — the fork notice (`Read-only built-in source · fork before
  publishing`) with Publish disabled for that reason, per production.
- **Publish-rejected strip** — the danger strip between canvas and dock
  (`Publish rejected — N issues to resolve.`).

### 2.3 · P3 — Decide the fate of the `_done/loops` pass

The finals link into `loops-catalog.html`, `loop-run-form.html`, `loop-configure.html`, and
`runs.html`, all pre-directive (old 48px topbar, badge-heavy, no Lucide). Options, in order of
preference:

1. **Refresh only the hero path**: `loop-run-form.html` (arrive-and-use is the product's hero
   flow) and `loops-catalog.html` (the entry point), promoted into `loops/`; leave configure
   and runs history archived until their specs move.
2. Leave all archived and accept the seam (state it in `index.html`'s foot).

Decision owner: Pedro. Don't refresh silently — it's scope.

## 3. The method (apply to every item above)

Per surface, in order — this is `DESIGN-LESSONS.md` §D operationalized:

1. **Transcribe first.** Read the production component(s) for the surface (or the closest
   pattern: `loop-run-inspect-sheet.tsx`, `MODAL-STANDARD.md`, listing toolbar) and note exact
   geometry. New event kinds/verbs come from the TechSpec tables, nothing invented (SD-007).
2. **Seed, don't regenerate.** Copy chrome from the approved final page of the same family.
3. **Apply the directive checklist:**
   - Lucide only (`<i data-lucide>` + CDN + `createIcons()`), one icon per concept, sized by container;
   - every section a `details/summary` (icon · title · one-line gist · chevron), calm defaults;
   - badge budget: state pills only, enums as text, zero counts render nothing;
   - canonical timeline anatomy for any event list;
   - machine truth demoted to micro mono, never removed;
   - every control gated by a payload field, cited in an HTML comment;
   - no explainer cards; hints/tooltips at point of use;
   - deltas annotated with authority (`production` / `spec` / `authorized delta`).
4. **Self-check + cross-links.** Keep the one-story rule: same loop (`software-delivery`),
   same run (`r-8f3a2b`), links resolving across the set; update `index.html` cards.
5. **Evidence.** PNG capture per artboard to `_captures/` — the exporter is blocked inside the
   design environment, so captures run repo-side via `eng-ui-screenshot`; until then, the user
   validates in the preview and the skip is stated explicitly.

## 4. Exit criteria (what "done" means for the whole set)

1. **Coverage matrices closed** — every event kind, verb, and parked state in the spec's
   `_prototype.md` tables maps to a designed treatment in a directive-compliant file.
2. **Approval log filled** — date + notes recorded in `_prototype.md`; that unblocks task_08
   (implementation MUST NOT start before it) and freezes the artboards as Visual Contract
   references for the `eng-ui-screenshot` bundles.
3. **Directives promoted** — `DESIGN-LESSONS.md` §D lands in its permanent homes:
   `design-system/GUIDE.md` (hard rules: Lucide, collapse anatomy, badge budget, canonical
   primitives) and repo institutional memory (`docs/_memory/` lesson for the
   production-transcription-first process). After promotion, the lessons file stays as the
   evidence record.
4. **README row truthful** — `opendesign/README.md` keeps pointing at `loops/index.html` and
   the index reflects the final ↔ lab split with no stale cards.
