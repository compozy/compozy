# Marketplace React Doctor remediation — 2026-09-14

## Scope and changes

All 18 diagnostics in the PR baseline were repaired: four React Compiler errors,
12 complexity warnings, one repeated batch lookup and one native-image warning.
Components now separate independently rendered sections; pure validation and status
projection are separate from hook orchestration. Async operations use supported Promise
cleanup while preserving failure reporting, scoped pending locks and batch progress.
Batch result lookup uses a map inside each profile/workspace group. Site raster icons use
Next Image with exact remote URLs derived from the published catalog snapshot.
No lint suppression, compiler opt-out, dependency change or compatibility shim was added.
The React Doctor PR workflow now fails on warnings as well as errors.

## Verification

- Pinned React Doctor 0.9.13: baseline four errors and 14 warnings, score 84;
  repaired PR comparison zero diagnostics, score 100. All three projects completed
  with no skipped checks (7 UI, 21 site and 218 Web files).
- A separate unfiltered scan of the 18 changed frontend files completed with no
  skipped checks: zero errors, zero warnings, score 100.
- Root Turborepo focused Web run: nine canonical suites, 160 tests passed. Coverage
  includes partial batch failures, pending cleanup, refresh errors, log identity,
  Settings navigation and secret-preserving edits.
- Root Turborepo site catalog suite: 48 tests passed. The added invariant is owned by
  the existing catalog suite: actual Next Image emits optimizer/responsive markup,
  and failed icon loading retains the existing fallback. That suite explicitly uses
  the real component instead of the global plain-image mock.
- Six 1440×900 browser captures were inspected: partial update progress, expanded
  degraded source, packaged installation inputs, required installation identifier,
  installed server detail and the real Next development-server Context7 page.
  Storybook states use controlled data; they do not replace daemon integration QA.
- The real site emits 48/96-pixel optimizer variants for the 40-pixel detail icon.
  All 17 feed icon URLs currently return upstream 404 because their files belong to
  this unmerged branch and the feed references main. The Context7 page recovered to
  its seeded logo. Successful remote optimizer delivery remains publication-dependent;
  no network interception or substitute image was used to claim success.

Local receipts are under `.deep-review/react-doctor-remediation/`; browser captures
are in `/tmp/marketplace-react-doctor.LT7j19/shots/`, outside Git history. Heavy gates
run in GitHub CI by explicit maintainer direction. Current-head CI status and links
are maintained in PR #636; this report does not infer a full gate pass from focused tests.

## Cross-surface impact

This follow-up updates the existing marketplace impact audit: Web component/hook
structure and public-site image delivery change; CLI/native tools, HTTP/UDS contracts,
configuration, extension SDKs/hooks, persisted data, scope isolation and official skill
instructions retain their already-approved contracts. No new migration or user copy is
needed. Existing public daemon QA remains separately documented in the review report.
