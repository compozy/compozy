---
id: ET-web-font-assets-strict-csp
area: ET
title: Bundled fonts load under the strict product CSP
persona: Dora
journey: J-operate-desktop-shell
expected: The production web bundle and packaged desktop app emit bundled WOFF2 files as same-origin assets, every declared UI font finishes loading, and the unchanged `font-src 'self'` policy reports no blocked `data:font` request in either surface.
entry_points: daemon-served web SPA; packaged CompozyOS app
qa_status: pass
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/runtime-ui-proof.md; /Users/pedronauck/dev/qa-labs/compozy-runtime-ui-regressions-20260827-155435-128437-lab/qa-artifacts/qa/font-csp-onboarding.png
last_report: docs/qa/reports/2026-08-27-runtime-ui-regressions.md
overlaps: ET-web-geist-wght-medium-510
---

QA impact 2026-08-27: Vite previously inlined small WOFF2 subsets as `data:` URLs while the
runtime CSP correctly allowed only same-origin font assets. The browser rejected those subsets and
logged a CSP violation. The build now opts every font format out of asset inlining without relaxing
the security policy. The walk must wait for `document.fonts.ready`, inspect failed font requests and
the first-page console, and repeat the check through the packaged Electron surface.
