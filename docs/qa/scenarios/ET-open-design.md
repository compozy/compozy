---
id: ET-open-design
area: ET
title: Design workspace HTML with the built-in open-design extension
persona: Ada
journey: J-extension-kit-lifecycle
expected: The open-design profile supports conversational HTML design and an explicitly requested independent review Loop using the OpenDesign-derived linter and curated local guidance, with workspace-contained files and accurate evidence.
entry_points: Profile open-design; session agent open-design-designer; ext__open_design__lint_artifact; loop open-design-review; extension inventory open-design
qa_status: blocked-verify
bug_ids: none
fix_status: pending
retest_status: pending
fix_commits: none
evidence: /Users/pedronauck/dev/qa-labs/compozy-open-design-20260911-203142-042642-lab/qa-artifacts/qa
last_report: 2026-09-11
overlaps: ET-ext-inventory; LP-runtime-validation-preflight
---

1. Boot an isolated current-build CompozyOS home. Verify `open-design` is a bundled enabled
   extension; its declared profile defaults to `open-design-designer`. Inventory and profile-scoped
   catalogs show two agents, three skills, one Loop, and the read-only lint tool. Confirm disabling, restarting, and re-enabling preserve
   the operator's choice and restore the same resources.
2. Invoke the public lint tool against actual HTML in the trusted workspace. Check an original P0,
   a clean brand-token control, original messages/severity, and the exact file digest. Confirm no
   mutation and reject traversal, outside workspace paths, symlink files/directories, and oversized
   HTML. A missing Node dependency is an error, never a passing check.
3. In a managed designer session, use a short feature request for bulk session actions. Observe
   local craft/reference reads and delivery under the requested `docs/design/` path. Open the HTML and inspect
   selection, action bar, confirmation, and local state changes. Confirm the session stays
   conversational, production source is untouched, and no review Loop starts implicitly.
4. Exercise a `_uiux.md` request and representative site, HTML-slide, and visual-document briefs.
   Check the selected artifact guidance and project design authority, related-state grouping, readable copy,
   local asset paths, and four appropriate HTML outputs. Do not count catalog presence as evidence
   that a deliverable was generated.
5. With agent-browser available, capture and inspect current rendering at an appropriate
   viewport; distinguish source checks from image inspection. Verify the agent reports browser
   unavailability without claiming a screenshot or adding a mandatory preview service.
6. Explicitly request `open-design-review` against an existing board. Validate and dry-run the
   published Loop with the skill's per-run limits (3 iterations, 2 revisions, full_body); confirm
   those effective values before starting, then observe designer → lint of every HTML → independent critic. Confirm a
   concrete correction is passed to the next complete generation, bounded to three passes, and
   that approval cites applicable checks and any source-backed exception. Terminal status and
   remaining findings must match durable run detail; no invented rollback or best-version claim.
7. Run the lab's exact teardown and retain clean process evidence.

## Current evidence — 2026-09-11

- Curated package and bundled lifecycle race suites pass with the existing installer. The local
  bundle passes all 97 original tests, and findings match on 106 original HTML examples.
- A real isolated daemon exposes the expected seven live resources. Public lint returns original
  findings and the exact file digest. Managed session `sess-07b905eb3b6fe45e` created a compact
  bulk-actions HTML and inspected interaction screenshots; desktop/mobile and confirmation were
  independently checked in an isolated browser.
- Session `sess-e2ecb4007e6cf2a2` checked the existing site/deck/document from `_uiux.md`. Parent
  verified rendering and slide navigation, but the site hero needs a wider text measure. The
  session selected global browser-use; the final skill now requires isolated agent-browser and
  the craft reference explains text-element measures. Those prompt changes await a real re-walk.
- Native per-run overrides verified `iteration_cap: 3`, `gate_max_revisions: 2`, and `full_body`.
  Run `looprun-76a2e94190969814` could not reach lint/critic because Claude hit its account limit;
  it was canceled. A successful independent review and correction generation remain unverified.
- The single requested Fable 5.1/xhigh code-review session `sess-726ee0230a5c74f7` hit the same
  provider limit before its verdict. Its retained history is not a completed review. No PR yet.
- The task-owned browser was closed; bootstrap teardown completed with `clean: true`.
- Final `make gate` passed all affected local Go and JavaScript lanes; CI is not yet available.
