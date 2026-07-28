# Coverage Taxonomy

Five dimensions every cycle must consider when deriving scenarios from flows. The taxonomy is a completeness *lens*, not a case generator: it answers "what kind of coverage did walking the flowcharts not give us?" — it never mandates a scenario per cell.

## Contents

- 1. Journeys
- 2. Functional checks
- 3. Experiential checks
- 4. Edge, error, and empty states
- 5. Cross-cutting
- Using the taxonomy in planning
- Anti-patterns

## 1. Journeys

Complete value paths, walked end to end: **Entry → Action → Result → Destination → Aftermath.**

- Entry: the user arrives the way real users arrive (link, nav, deep link) — not by pasting an internal URL.
- Result: the immediate observable of each action.
- Destination: where the flow actually lands the user, including click-throughs from emails/notifications.
- Aftermath: the true end state — the record exists on fresh load, the side effect landed correctly, the journey survives refresh and deep-link.

This dimension is the primary one; the other four qualify it.

## 2. Functional checks

Do the mechanics hold along the journey?

- Forms validate and preserve input on error.
- Links and buttons resolve to where they claim.
- Data round-trips: what was saved is what reloads.
- The console/log surface is clean of errors during the walk.
- Auth boundaries hold: signed-out and wrong-role access behaves correctly.

Functional checks live *inside* journey scenarios — they're what "expected observable" means at each step, not a separate suite.

## 3. Experiential checks

Would a real person *enjoy* — or at least not resent — this walk?

- Does the surface match the product's intent and voice? Copy in user language?
- Loading, transition, and skeleton states: does the product feel alive or stalled?
- Persona paper cuts: friction too small to fail a functional check but real enough to degrade the experience (defined in the `qa-execution` skill's session protocol).
- The six experiential lenses (the `qa-execution` skill's lenses reference): usability, accessibility, perceived performance, compatibility, error recoverability, production parity.

## 4. Edge, error, and empty states

The realistic ways a walk goes sideways:

- Error branches from the flowchart (validation, permission, conflict) — each with recoverable UX.
- Empty states: first-use, zero-results, cleared-data.
- User-behavior edges (the `qa-execution` skill's edge-case catalog): refresh mid-submit, back after success, double-click, autofill, expired session, flaky network.
- Abandon-and-resume paths from the journey map.

## 5. Cross-cutting

Qualities no single journey owns:

- **Responsiveness:** the journey at 375 / 768 / 1280 viewports when layout was touched.
- **Regression:** adjacent journeys that share components/services with the change — the canary walks.
- **Consistency:** same nouns, same icons, same patterns across the touched surfaces.
- **Continuity:** cross-device and cross-session (start on phone, finish on laptop) where the product promises it.

## Using the taxonomy in planning

After deriving scenarios from flows (routed at Step 3 of the SKILL):

1. Sweep the five dimensions per journey in scope.
2. For each dimension either (a) point at the scenario/charter that covers it, or (b) record a deliberate skip with reasoning in the cycle's notes.
3. A dimension that is neither covered nor consciously skipped is a planning gap — fix the plan, don't pad the tracker.

## Anti-patterns

- **Cell-filling** — generating a scenario per dimension per journey mechanically. The taxonomy qualifies coverage; it doesn't manufacture it.
- **Dimension suites** — splitting the five dimensions into five separate test suites recreates the technical-case model. Dimensions ride along journeys.
- **Silent skips** — a skipped dimension without recorded reasoning reads as covered. It isn't.
