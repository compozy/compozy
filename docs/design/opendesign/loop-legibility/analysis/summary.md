# Summary: loop-legibility design exploration

## Research Question

What visual contracts, incumbent grammar, production seams, and canonical data must the loop-legibility design set honor so the six artboards under docs/design/opendesign/loop-legibility/ can be built without inventing states, copy, chrome, or entities? Extract only design-actionable facts for S1 (tasks list), S4 (run default + needs-you), S5 (DAG + roster), and S6 (runs roster).

## Slice Map

| NN | Slug | Slice question | One-line finding |
| --- | --- | --- | --- |
| 01 | brief-and-states | What must each of the six artboards contain — states, data, vocabulary, VC ids? | Six files only; two registers, no toggle; one two-loop dataset; every lab panel maps to a `task_04`/`task_05` VC row. |
| 02 | production-surfaces | What live chrome, verbs, and seams must boards reuse or omit? | Keep 1240/320, 44px head, closed verb/status sets; today's 9-section cockpit is the *incumbent to replace*, not the target. |
| 03 | design-grammar | What locked visual language must every board inherit? | Continue graph-eng: request→tone map, wait sentences, `approve/edit/reject/respond`, labs ≤1240 + 320px rail, `Pill` not `Badge`. |

## Convergences

- **Two-register run page, Inspect, never a mode toggle.** ADR-002 (`01`) plus production Inspect-as-sheet (`02`) plus graph-eng disclosure chrome (`03`). Failure and needs-you stay in the default register.
- **1240 × 320 geometry is production truth.** `loop-run-page-body.tsx:255-256` (`02`), `--req-win-w` / `--req-rail-w` (`03`), `_uiux.md` lab rule (`01`).
- **Needs-you on the page is warning; danger stays failed/quarantined + bell.** `_uiux.md` incumbent grammar (`01`) matches graph-eng DESIGN-NOTES (`03`). Do not leak ADR-006 re-ink onto S4/S6.
- **Decision words are exact:** `approve / edit / reject / respond`. Gate verbs (`Approve & resume` / `Request changes` / `Reject & halt`) stay a different triad (`02`) — do not merge bars.
- **Fan-out is a rollup chip, never per-item nodes.** US-011.EC-1 (`01`) + graph-eng progress chapter (`03`) + production segment-bar rule (`02`).
- **`pending` ≠ `not_taken`.** Safety Invariant 14 (`01`); both are neutral but distinct glyphs (`03`).
- **Canonical fixture is `revisao-paralela` + `fabrica-assistida`.** `_uiux.md` / `_dx.md` (`01`) override graph-eng's older `release-train` / `run_7f3a` story (`03` transferable note).
- **Set files:** `DESIGN-NOTES.md` + chaptered CSS + 880px `index.html` + final+lab boards (`03`). Artboard CSS is a contract, never imported into production.

## Divergences

- **Slice 02 describes today's cockpit; slice 01 specifies the redesign.** Production stacks 9+ named sections and has no run-bound DAG. `_uiux.md` S4 replaces that stack with briefing → needs-you → progress → story, Usage+About on the rail, everything else behind Inspect. **Spec wins.** Reuse from 02: widths, topbar crumbs/verbs, closed status/verb sets, request vs gate vocabularies, kind glyphs. Do not keep Happening now / Waiting / strategy / Waits rail as default-read competitors.
- **Slice 02 reads S5 as the definition DAG + recent-runs table.** `_uiux.md` S5 is the *new* operator register on the run page (live DAG + complete node roster + generation history). Definition DAG stays definition-only (`loop-body-dag.tsx`).
- **S6 columns.** Production: Outcome · Loop · Inputs · Gens · Best · Started · Budget. Spec: Loop · Status/needs-you · Progress · Started · Duration. **Spec wins.** Gens/Best/Budget demote to the run page. Four KPI tiles are not in `_uiux.md` S6 — omit from the default roster read (they inflate a calm list).
- **Usage rows.** Production usage rail is Time / Tokens / Cost / Rounds. Spec Usage is tokens · cost · budget · rounds · duration. **Spec wins** for the default rail; keep production warn-at-90% / danger-at-ceiling behavior.
- **COPY.md has no status map** (`03`). Status words come from production formatters (`02`) + `_dx.md` briefing sentences (`01`) + graph-eng request words (`03`).

## Risks & Open Questions

Parent resolutions (locked for the design pass):

| Question | Resolution |
| --- | --- |
| S4 reduced-motion has no VC | Record in DESIGN-NOTES; only stage `task_05/VC-24` on the DAG board (pulse unmounts). |
| Budget exhausted has no VC | Extra lab panel on `loop-legibility-run-default.html`, labeled design-lab (not a VC id). |
| US-014 conflict / stale-view | Implementation toast — not staged. |
| `fabrica-assistida` has no run id | Omit the id. Row leads with loop name + started 18:41 + 13m. |
| Live-run 9m40s vs 22m | Usage rail uses briefing `9m40s`; S6 row uses roster `22m`. |
| ADR-002 four questions vs spend | Usage stays **rail-only**. Main column stays four elements. |
| Wait-kind sentences | Exact graph-eng strings. |
| Kind glyphs | Additive inventory from `loop-palette.ts` / `loop-node-kind-icons.ts`. Boards may stage a subset as story. |
| New CSS vs append graph-eng | New `loop-legibility.css`. Link `graph-eng.css` for request chapters 1–5. New chapters append here; never restyle 1–12. |
| S6 primitive | `ListingRow` + table. Not `RunCard`. |
| S6 filter chips | No new taxonomy. Server order (needs-you → active → terminal) is the design. Do not reuse loops_v2 inventory filters. |
| Chapter 0 parity | Yes — rebind `--subtle` / `--faint` to production oklch. |
| S5 DAG geometry | Run-page fluid ≤1240, not editor full-bleed. |
| `Badge` / `Disclosure` | Use `Pill` and `Section` / `Collapsible`. |
| Host titles | Window `Tasks` / crumbs `Loops → Runs → {loop_name}`. |

Remaining risks (do not regress):

- Color-alone chips (WCAG).
- Leading default reads with `looprun-…` ids (E2E-012).
- Persisting the S1 reveal filter.
- Editor chrome on the run DAG.
- Mixing gate verbs with request verbs.
- Inventing a seventh board for S2/S3/S7.
- Inventing `fabrica-assistida` topology or a third loop.

## Recommended Next Steps

1. Lock `DESIGN-NOTES.md` with the resolutions above plus the graph-eng needs-you tone confirmation (`01`, `03`).
2. Scaffold `loop-legibility.css` (chapter 0 parity + set chrome) and `index.html` hub (`03`).
3. Build S4 first — `loop-legibility-run-default.html` then `loop-legibility-needs-you.html` — it is the briefing-test surface and the largest redesign (`01`, `02` divergence).
4. Build S6 `loop-legibility-runs-roster.html` with server order and spec columns (`01`).
5. Build S1 `loop-legibility-tasks-list.html` — default work-only, then reveal labs (`01`, `02` list grammar).
6. Build S5 `loop-legibility-run-dag.html` + `loop-legibility-run-roster.html` as disclosed operator depth (`01`, `03` DAG geometry).

## Index

- `docs/design/opendesign/loop-legibility/analysis/01_analysis_brief-and-states.md`
- `docs/design/opendesign/loop-legibility/analysis/02_analysis_production-surfaces.md`
- `docs/design/opendesign/loop-legibility/analysis/03_analysis_design-grammar.md`
