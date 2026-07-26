---
id: ET-web-inter-opsz-medium-510
area: ET
title: Inter Variable opsz + UI medium 510
persona: Bruno
journey:
expected: Runtime web and Storybook load Inter Variable with the opsz+wght axes; body uses font-optical-sizing auto; every font-medium surface resolves to weight 510; `--text-small-body` is 12.5px (0.78125rem); UI titles and rows keep the DESIGN.md tracking ladder (detail-h1 / tight / body) so type density matches OpenDesign prototypes.
entry_points: web SPA (`web/src/styles.css`); Storybook (`packages/ui/.storybook/preview.css`); site (`packages/site/app/layout.tsx` Inter axes); tokens `--font-weight-medium`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps:
---

Added by typography alignment with OpenDesign (2026-07-20). Flag only — retest in the next QA cycle.

Verify against `docs/design/opendesign/shell/agh-refined-7.html` and `DESIGN.md` §3: Inter Variable opsz, UI medium 510, body tracking −0.006em, features cv01/ss03/cv11.
