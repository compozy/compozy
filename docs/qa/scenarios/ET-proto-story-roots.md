---
id: ET-proto-story-roots
area: ET
title: Browse Web and shared UI stories in the prototype gallery
persona: Bruno
journey: J-operate-desktop-shell
expected: Both story roots render in light and dark, retain their Storybook previews and interactions, and leave existing prototype and flow navigation working.
entry_points: bun run proto dev; /?components
qa_status: pass
---

Run from the repository root with the supported Storybook peers installed in both packages.
Use a free gallery port when another worktree is active. This gallery uses fixtures, without a daemon.

1. Run `bun run proto check --json`. Expect no problems; module-mock and uncovered-state warnings
   are separate from errors.
2. Open the gallery overview, then Components. Expand both `web` and `ui`, open a component and
   navigate between its stories. Check dark and light themes, including command-palette Open,
   marketplace states, an app-router story with MSW, and CodeBlock CopyInteraction.
3. Run `bun run proto stories run --root web --theme dark --json`, then repeat for light and for
   `--root ui`. Retain all four reports and the revisions. Compare ids and failure stages with
   an unmodified Storybook baseline. The stories spec permits only
   `systems-marketplace-routes-marketplace--logo-generator-failure` at `load`, because `sb.mock`
   is inert. Record other failures individually; never count an incomplete run as passing.
4. Open existing prototypes (`agent-avatar`, `sessions-board`, `session-pull-request-chip`) and
   both flows. Confirm their selected screens render and their navigation remains available.
5. Stop every server started by this walk, preserving other worktrees' servers.

Acceptance owner: proto-stories `_tests.md`, GATE-001/GATE-002. The development validation uses
screenshots and JSON reports under `/tmp/proto-validate/`; it is not a substitute for the complete
Storybook build baseline required by those gates.

Consumer impact follows the proto-stories spec's Impact Analysis: Web-only gallery configuration,
optional Storybook peers per root, and a presentation-only UI Vite entry. No native tool, daemon
hook, persisted workspace data, config.toml, or official Compozy skill changes are required.

2026-11-04 round 1 development acceptance: all four root/theme corpora were collected. The walk fails:
controlled loss of the real Chromium browser aborts `proto stories run` without per-story rows.
A page-only loss records its last stage and continues. Proto code was left unchanged under the
consumer-validation boundary. Local evidence and unabridged failures: `/tmp/proto-validate/report.md`
and `/tmp/proto-validate/failures.md`. Full Storybook-build parity remains unverified.

2026-11-04 round 2, proto b2f0dd5: all four cold corpora completed in one CLI invocation each,
and all four unmodified static Storybook browser baselines were collected. Web/light and both UI
themes passed the parity comparator. Web/dark failed its six import-timeout rows; those six passed
a focused rerun, but the original cold failures remain. All 112 failing CLI rows have evidence:
110 also fail on plain Storybook, and two are the expected module-mock exception. Real browser-loss
continuation and occupied-port exit regressions passed. Local report: `/tmp/proto-validate/r2/report.md`.

2026-11-04 round 3, proto 5a7a115: all four cold corpora completed in one CLI invocation each.
All six prior Web/dark import timeouts passed. Web dark/light and UI/light passed parity against
the unchanged round-2 static Storybook baselines. The single UI/dark parity row,
components-custom-entitydialogheader--focus-visible-close at play, has disposition (b) consumer
race, parity 4/4 PASS: the story asserted focus before Base UI's FloatingFocusManager restored it
in a requestAnimationFrame callback. Three cold UI/dark reruns passed with zero parity violations,
and a paired 60-sample stress run at 4x CPU failed 9/60 in proto and 23/60 in static Storybook
10.6.3. The story now waits for focus before asserting it. Final classification: (a) 0, (b) 108,
(c) 2, (d) 0. Build/typecheck passed, proto check had zero problems, and gallery/prototype/flow
checks passed. Evidence, in the compozy/proto workspace under
`.compozy/tasks/proto-stories/qa/`: `validation-r3/report.md`,
`validation-r3/ADDENDUM-disposition.md`, and `focus-race/` (`report.md`,
`before-{0,1,2}.parity.json`, `matched-focus-{proto,static}.json`). All task servers are stopped.
