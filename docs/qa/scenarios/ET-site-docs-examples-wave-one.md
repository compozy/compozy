---
id: ET-site-docs-examples-wave-one
area: ET
title: Docs Examples section ships runnable artifacts with honest maturity labels
persona: Dora
journey: J-evaluate-compozy-beta
expected: /docs/examples appears in the sidebar under Guides & examples between Guides and Use cases with a FlaskConical icon, and lists five wave-one pages. Every example page follows the same anatomy — What you build, The artifact, Run it, How it works, Next steps — and carries a maturity chip in its masthead. The two Loop pages fence the exact `extensions/spec-cycle/loops/<name>/loop.yaml` shipped in the repository, copyable as one block. Every command shown is one the runtime accepts, and no page documents a mechanism that does not ship (no file-watch triggers, no invented config keys).
entry_points: compozy.com /docs/examples; /docs/examples/review-and-fix-loop; /docs/examples/implement-tasks-loop; /docs/examples/morning-briefing-job; /docs/examples/webhook-to-agent-run; /docs/examples/react-to-session-end
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence: docs/qa/evidence/2026-08-14-review-handoff-spec-cycle/site-examples-index.png;docs/qa/evidence/2026-08-14-review-handoff-spec-cycle/site-orchestrate-example.png
last_report: docs/qa/reports/2026-08-14-review-handoff-spec-cycle.md
overlaps: ET-site-docs-single-tree-ia; ET-site-docs-sidebar-opendesign; ET-spec-cycle-skill-bundle
---

Added 2026-07-29 with the site IA gap closure (spec `.compozy/tasks/site-docs-ia/_spec.md` §8.3,
wave 1): the docs had 594 runtime pages and not one runnable copy-paste artifact. The section ships
the three real spec-cycle Loops as walkthroughs plus three automation artifacts grounded in
`internal/automation` — a cron job, a signed webhook, and a `session.stopped` observer trigger.

Artifact parity is gated in code: `lib/__tests__/runtime-docs-truth.test.ts` asserts the fenced YAML
in each Loop page byte-matches the shipped `loop.yaml`, so drift fails the site test lane rather than
shipping a stale example. The QA walk owns what that test cannot see — that the rendered anatomy is
consistent across the six pages, the maturity chip renders, the copy affordances copy the real text,
and each documented command exists in the generated CLI reference.

Wave 2 (spec §8.3) is out of scope for this scenario; add its own when those artifacts land.

QA impact 2026-07-29 deep-review remediation: reset after runnable commands were corrected to use
required flags, generated IDs, real trigger output fields, and explicit agent/provider prerequisites.

QA impact 2026-08-14: reset because the `orchestrate-tasks` walkthrough joined the section. The next
walk must verify the six-page index, all three Loop examples, and the new page's copyable artifact.

QA result 2026-08-14: passed on the local production-shaped docs site. The Examples index rendered
all six entries, the orchestrate-tasks page preserved the shared anatomy and copy affordance, and its
artifact exposed the strict frontmatter parser plus starting/active/stopping worker checks after reload.

QA impact 2026-08-26: reset because the standalone orchestration walkthrough was removed and the
`implement-tasks` example now documents both modes and nine inputs in one byte-exact artifact.
