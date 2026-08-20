---
id: ET-web-extension-union-install
area: ET
title: Install an extension from the source union in the web
persona: Bruno
journey: J-extension-distribution
expected: The Extensions-only "Install extension" entry point submits the generated `InstallExtensionRequest` union shape (`{source, ref, version?, asset?, allow_unverified?}`) for local_path, github, and git; per-source ref grammar is validated inline before the request; unverified archives route through the daemon's explicit consent dialog (including the daemon's own 422 reason); daemon validation errors are surfaced verbatim; and the curated one-click card install stays unchanged.
entry_points: /marketplace/extensions (Install extension); `POST /api/extensions`
qa_status: untested
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: web/src/systems/marketplace/components/extension-install-dialog.tsx;web/src/systems/marketplace/components/extension-install-model.ts;web/e2e/__tests__/extensions.spec.ts;/Users/pedronauck/dev/qa-labs/compozy-ext-improvs-final-20260729-230047-267985-lab/qa-artifacts/qa/browser/extension-management.json;/Users/pedronauck/dev/qa-labs/compozy-devtool-oss-launch-20260802-195112-911343-lab/qa-artifacts/qa;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/evidence/extensions-closeout.json;/Users/pedronauck/dev/qa-labs/compozy-go-modernization-closeout-20260804-121411-946266-lab/qa-artifacts/qa/screenshots
last_report: docs/qa/reports/2026-08-04-go-modernization-closeout.md
overlaps: ET-web-extensions-manage; ET-extension-published-source-installs
---

Added by ext-improvs Task 08. Cover all three sources against one real daemon: a relative local path
rejected before the request, an absolute path installed after consent, `owner/repo[@tag]` grammar,
and a credential-bearing git URL rejected inline.

QA impact 2026-07-29: new surface. Never verified against a real daemon.

Visual recapture was explicitly waived by the operator on 2026-07-29. Functional browser coverage
remains the implementation evidence; this scenario stays `untested` for the next QA cycle.

QA impact 2026-08-03: the Git source form now accepts only credential-free HTTPS repository URLs,
routes branch/tag/commit input to Version, and rejects query or fragment syntax inline. Re-walk the
real daemon flow and capture the hint plus each recovery message before completion.

QA 2026-08-04: passed through the normal Marketplace entry point against the isolated daemon. The
dialog exposed local, GitHub, and Git source members; Git kept Version separate and rejected HTTP,
SSH, credentials, query, and fragment syntax inline with specific recovery text. A local unverified
extension required explicit consent, became active, appeared in Installed, and was removed after the
canary. The official bundled `compozy` skill remained visible. Desktop and narrow captures plus every
Git recovery state are linked above.
