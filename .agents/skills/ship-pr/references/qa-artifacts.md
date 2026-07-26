# QA artifacts in the PR description

## Contents

1. When this section applies
2. Where artifacts live
3. What to extract
4. PR-body `## QA` template
5. Examples
6. Pitfalls

## 1. When this section applies

Include a `## QA` section in the PR description only when the pre-flight detector reports a non-empty `qa_output_paths` array. Those paths are produced by the `qa-report` and `qa-execution` skills (or any equivalent flow). When none are present, **omit the QA section entirely** — do not write "no QA performed", which reads as a red flag to reviewers.

The detector accepts a manual override via the `QA_OUTPUT_PATH` environment variable for repos that keep QA artifacts somewhere non-standard.

## 2. Where artifacts live

The living-docs convention (current, `compozy/agh` and sister repos):

```
docs/qa/
├── scenarios/<AREA>-<slug>.md       # source-of-truth scenario tracker
├── reports/<YYYY-MM-DD>-<scope>.md  # dated run reports (Final Status, matrix, fixes)
├── bugs/BUG-<YYYYMMDD>-<slug>.md     # global bug registry
├── charters/CH-<slug>.md             # immutable session charters
└── evidence/<date>-<scope>/         # checkpoint/failure screenshots
```

`state.csv` may exist locally as a gitignored generated view; scenario files are authoritative.

Legacy per-round trees (`.compozy/tasks/<slug>/qa/` with `verification-report.md`, `issues/`, `test-cases/`) may still exist in older branches; the detector falls back to them when no `docs/qa/` is present.

Probe before reading:

```bash
QA_DIR="docs/qa"
latest_report=$(ls "$QA_DIR/reports" 2>/dev/null | sort | tail -1)
test -n "$latest_report" && head -60 "$QA_DIR/reports/$latest_report"
ls "$QA_DIR/bugs" 2>/dev/null | wc -l
```

## 3. What to extract

For the PR body, pull these signals — prefer the latest dated report relevant to the branch (match `<scope>` to the branch slug):

- **Final Status** — the report's `## Final Status` verdict line (ready / not ready / ready with blocked items) plus the issues-by-user-impact totals (Blocks-Completion / Data-Loss / Trust-Damage / Friction / Cosmetic).
- **Bug references** — the `BUG-<YYYYMMDD>-<slug>` ids filed or updated by this run (from the report's matrix `Issue` column); link the registry files. Grandfathered counter ids remain valid when a report cites them.
- **Session coverage** — from the report's Session Matrix: which personas walked which journeys/charters.
- **Screenshot evidence** — pick 2-5 representative PNGs from `docs/qa/evidence/<date>-<scope>/`. Use repo-relative paths so GitHub renders them inline.
- **Gaps and blocked items** — the report's `Human Verifications Needed` and `Decisions for a Human` sections, plus any disclosed skips.

## 4. PR-body `## QA` template

```markdown
## QA

**Final Status:** <verdict sentence from the report>

**Coverage:** <one sentence — personas × journeys walked, e.g. "3 personas across 5 journeys, Money + Back-Button tours">

**Issues:** <total> (Blocks-Completion: N, Data-Loss: N, Trust-Damage: N, Friction: N, Cosmetic: N) — see `docs/qa/bugs/`.

**Evidence:**
- ![onboarding-step3](docs/qa/evidence/<date>-<scope>/CH-012-step3.png)

**Blocked / needs human:** <copy from the report, or "None">

**Full report:** [`<date>-<scope>.md`](docs/qa/reports/<date>-<scope>.md)
```

## 5. Examples

**Living-docs run:**

```markdown
## QA

**Final Status:** ready with blocked items — 1 leg needs human verify (real payment).

**Coverage:** 3 personas (Marina/mobile, Rui/casual, Ana/screen-reader) across checkout + settings journeys, Money and Back-Button tours.

**Issues:** 3 (Blocks-Completion: 0, Data-Loss: 0, Trust-Damage: 1, Friction: 2) — BUG-20260705-coupon-lost fixed (regression test `web/e2e/checkout-coupon.spec.ts`), two registry bugs open.

**Evidence:**
- ![money-tour-coupon](docs/qa/evidence/2026-07-05-checkout-v2/CH-031-step4.png)

**Blocked / needs human:** Verify the live Stripe charge on a real card (report row #7, exact steps in the report).

**Full report:** [`2026-07-05-checkout-v2.md`](docs/qa/reports/2026-07-05-checkout-v2.md)
```

**Legacy-tree fallback (older branches):** extract totals from `verification-report.md` and bug counts from `qa/issues/`, and say the run predates the living-docs tree.

## 6. Pitfalls

- **Do not stage evidence into the PR commit unless intended to ship.** `docs/qa/` trackers/reports are normally committed; per-run evidence may be gitignored — verify before committing.
- **Do not paste base64-encoded screenshots into the PR body.** Use repo paths or GitHub-uploaded attachments.
- **Do not re-derive totals the report already states.** The report's Final Status block is authoritative; recount only when no report exists.
- **Do not link absolute paths** (`/Users/...`). Use repo-relative paths so the links work for everyone.
