---
id: ET-web-extension-union-install
area: ET
title: Install an extension from the source union in the web
persona: Bruno
journey: J-marketplace-acquisition
expected: The Extensions-only "Install extension" entry point submits the generated `InstallExtensionRequest` union shape (`{source, ref, version?, asset?, allow_unverified?}`) for local_path, github, and git; per-source ref grammar is validated inline before the request; unverified archives route through the daemon's explicit consent dialog (including the daemon's own 422 reason); daemon validation errors are surfaced verbatim; and the curated one-click card install stays unchanged.
entry_points: /marketplace/extensions (Install extension); `POST /api/extensions`
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: web/src/systems/marketplace/components/extension-install-dialog.tsx; web/src/systems/marketplace/components/extension-install-model.ts; web/e2e/__tests__/extensions.spec.ts
last_report:
overlaps: ET-web-extensions-manage; ET-extension-published-source-installs
---

Added by ext-improvs Task 08. Cover all three sources against one real daemon: a relative local path
rejected before the request, an absolute path installed after consent, `owner/repo[@tag]` grammar,
and a credential-bearing git URL rejected inline.

QA impact 2026-07-29: new surface. Never verified against a real daemon.

Visual recapture was explicitly waived by the operator on 2026-07-29. Functional browser coverage
remains the implementation evidence; this scenario stays `untested` for the next QA cycle.
