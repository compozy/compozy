---
id: ET-open-design
area: ET
title: Design workspace HTML with the built-in open-design extension
persona: Ada
journey: J-extension-kit-lifecycle
expected: The open-design profile supports conversational HTML design and an explicitly requested independent review Loop using the OpenDesign-derived linter and curated local guidance, with workspace-contained files and accurate evidence.
entry_points: Profile open-design; session agent open-design-designer; ext__open_design__lint_artifact; loop open-design-review; extension inventory open-design
qa_status: pass
bug_ids: none
fix_status: fixed
retest_status: pass
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
5. With the uniquely named `open-design-browser` skill and agent-browser CLI available, capture and inspect current rendering at an appropriate
   viewport; distinguish source checks from image inspection. Verify the agent reports browser
   unavailability without claiming a screenshot or adding a mandatory preview service.
6. Explicitly request `open-design-review` against an existing board. Validate and dry-run the
   published Loop with the skill's per-run limits (3 iterations, full_body); confirm
   those effective values before starting, then observe designer → lint of every HTML → independent critic → native digest verification. Confirm a
   concrete correction is passed to the next complete generation, bounded to three passes, and
   that approval cites applicable checks and any source-backed exception. Terminal status and
   remaining findings must match durable run detail; no invented rollback or best-version claim.
7. Run the lab's exact teardown and retain clean process evidence.

## Current evidence — 2026-09-11

- Curated package and bundled lifecycle race suites pass with the existing installer. The local
  bundle retains all 97 original tests. After external-review corrections, 108 cases pass;
  92 of 106 original examples remain identical, while 14 change only for corrected raw-color
  scanning across every style block.
- A real isolated daemon exposes the expected seven live resources. Public lint returns original
  findings and the exact file digest. Managed session `sess-07b905eb3b6fe45e` created a compact
  bulk-actions HTML and inspected interaction screenshots; desktop/mobile and confirmation were
  independently checked in an isolated browser.
- Final `_uiux.md` session `sess-39e3ad66d4f5b2db` refined the site, slides, and one-page document,
  used isolated agent-browser, and loaded actual images. Desktop 1440px, mobile 375px, keyboard
  interaction, slide navigation, and print layouts were checked. Four-format public lint returned
  zero findings with file digests. Evidence: the original lab's `lint-four-formats-final.json`,
  session history, and final screenshots. The PDF generator used Letter despite the document's
  A4 CSS; that print-environment limitation is retained in the evidence.
- The single requested Claude Fable 5.1/xhigh code review, `sess-726ee0230a5c74f7`, completed.
  Its corrections include fail-closed typed critique, bounded completion cost, a unique browser
  skill name, site inventory, subprocess diagnostics, and canonical codegen drift checks.
- Native preflight now exercises executed snapshot construction and hydration in the existing
  resource suite. The live dry-run passed with `iteration_cap: 3` and `full_body`; the critic is
  an isolated tool-capable agent and final lint verifies unchanged file digests.
- Final run `looprun-8878f4f32a5fa63e` completed after two generations: the independent critic
  found a clipped heading, the designer repaired it, and final critique approved with zero blockers.
  All four nodes succeeded; native verification and the on-disk file had the same SHA-256.
  Evidence: `review-final-receipt.json` and the final critic history.
- Selected-profile Web catalog/detail/picker/reload and authored-agent duplicate/edit/delete
  passed. The initial HTTP walk covered 19 steps; its AGENT.md update-denial premise was corrected in round one to preserve supported extension customization.
  The final canonical gate passed, including 7,245 Web tests; production Web and Site builds passed.
  The continuation lab's strict evidence audit and exact teardown passed with `clean: true`.
- Original lab teardown recorded `clean: true`. Continuation evidence is under
  `/Users/pedronauck/dev/qa-labs/compozy-open-design-20260911-203142-042642-20260911-223928-759363-lab/qa-artifacts/qa`.

Round-one external-review verification (2026-09-11): the final native linter reports zero findings
for all four unchanged generated HTMLs. The shipped Loop dry-run compiles with exact sourced
exception coverage, path/SHA association and the explicit three-generation/full-body limits. Its
real compile/hydration/schema/CEL suite rejects missing/unrelated exceptions and stays within the
existing cost budget at 7,810/10,000 for 32 files and 512 findings. The earlier provider-backed
Loop run remains evidence for unchanged design/critique interaction, not a replay of this new
exception schema. Profile-specific authored-context and editable bundled-agent re-walks are
recorded in RT-077 and ET-profile-extension-agent-skill-isolation. Current lab evidence:
`/Users/pedronauck/dev/qa-labs/compozy-open-design-r1-20260911-234610-059353-lab/qa-artifacts/qa`.

Round-one final strict audit passed without blockers or warnings; the local gate passed all
affected lanes (7,277 Web tests). Lab teardown completed with `clean: true` and no surviving
owned process.
