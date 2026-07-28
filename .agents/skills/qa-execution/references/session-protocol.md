# Session Protocol

How one persona walks one journey and produces evidence someone else can audit. The loop is always: **enter as the persona → act → observe → verify → capture → continue**. The browser is the instrument — it cannot be talked into agreeing, which is exactly why its readings are the proof.

## Contents

- Adopting the persona profile
- The core loop (browser)
- The evidence standard
- Paper cuts
- Session logging
- CLI / HTTP journeys
- Browser command surface (`agent-browser`)
- Anti-patterns

## Adopting the persona profile

Before the first interaction, materialize the charter's persona physically:

- **Viewport/device:** `--viewport 375x812` for phone-small, etc. — sessions per profile, not one desktop window for everything.
- **Network:** throttle to the persona's reality (slow 3G for the mobile persona) when the tooling allows; otherwise record the gap.
- **Locale/timezone:** set to the persona's, especially for Locale-tour charters.
- **Entry:** arrive the way the persona arrives — the email link, the landing page, the deep link. Never paste an internal URL the persona wouldn't have.
- **Auth:** the real auth path the persona uses (SSO, magic link). Save state (`agent-browser state save`) to reuse across sessions *of the same persona only*.

## The core loop (browser)

For every journey step:

1. **Snapshot** the interactive state: `agent-browser snapshot -i`.
2. **Act** as the persona would — click, fill, scroll. A New User doesn't know the shortcut; a Power User always uses it.
3. **Re-snapshot** — element refs go stale after DOM changes; never act on a stale ref.
4. **Verify** the step's expected observable within the persona's patience window. Note the time-to-feedback for anything that should feel instant.
5. **Capture** at checkpoints (see evidence standard); log the step verdict (`pass` / `friction` / `fail`).
6. **Branch** where the flow branches: follow the charter's marked branch and abandonment paths, recording what resuming looks like.

Semantic waits (`wait --text`, `wait @ref`) over fixed sleeps; chained commands only when intermediate output isn't needed.

## The evidence standard

A step or scenario is `Pass` only when ALL of:

1. **The action produced its observable in the UI/CLI** the persona used — visible, in user language.
2. **An independent read path confirms it.** A fresh load, a different surface (the list view, the email, the API the product itself exposes publicly), or a second session shows the same state. Optimistic UI is not confirmation.
3. **It survives refresh and deep-link.** Reload the page; revisit by URL. State that evaporates was never saved.
4. **Evidence is captured**: screenshot at the goal state and at every divergence, with paths recorded in the session log.

Route-renders and list-counts are smoke, not proof: "the page loaded" and "there are 3 items" only count when tied to the specific object this session created. The default assumption for any anomaly is **product bug until disproven** — never "probably my environment" without checking.

Screenshots go to `<qa-docs-path>/evidence/<report-slug>/<charter>-<step>.png` — checkpoints and failures only, per the evidence policy of the QA docs layout (owned by the `qa-report` skill).

## Paper cuts

Paper cuts are the second judgment of every session: friction too small to fail a functional check but real enough to degrade the experience. The functional walk answers "does it work?"; the paper-cut hunt answers "would this persona come back?"

- Capture in the persona's words: *"a mobile user with one hand can't reach submit"*, not *"button 8px below fold"*.
- Rate each: **sharp** (the persona would complain or hesitate to return) or **dull** (noticed, shrugged off).
- Sharp paper cuts enter the fix loop as findings (usually `Friction` or `Trust-Damage` tier); dull ones go to the report's Paper Cuts section for pattern-watching.
- Paper cuts are a lens, not a second instrument: the same mind re-reading its own walk. Trust the browser's readings over the persona's feelings when they disagree — and record both.

## Session logging

Log every session as it runs (this feeds the session debrief in the report):

```yaml
- charter: CH-<slug>
  journey: J-<slug>
  persona: <name>
  entry: <how the session entered>
  steps:
    - step: 1
      attempted: <verb>
      observed: <what actually happened>
      evidence: <path, when captured>
      time_to_feedback_s: <observed, when relevant>
      verdict: pass | friction | fail
  goal_reached: yes | no | partial
  true_end_state: confirmed | not-confirmed | blocked (<why>)
  abandonment_paths: [<which were followed, outcome>]
  paper_cuts:
    - felt: "<persona language>"
      sharpness: sharp | dull
  bugs: [BUG-<YYYYMMDD>-<slug>]
  scenarios_settled: {<id>: <qa_status>}
```

## CLI / HTTP journeys

Products without a web surface (or with CLI/API personas) walk the same protocol: drive the workflows through the interfaces a real operator uses — the installed CLI, the documented API — never internal test helpers. Capture the exact command, input, and observable output per step; the independent read path is a second verb or endpoint that shows the same state.

## Browser command surface (`agent-browser`)

```bash
# Navigation
agent-browser open <url> | back | forward | reload | close

# Snapshot & interaction
agent-browser snapshot -i               # interactive elements with refs
agent-browser click @e1 | fill @e2 "text" | select @e3 "v" | press Enter
agent-browser check @e1 | uncheck @e1

# Evidence
agent-browser screenshot <path.png> [--full]
agent-browser get text @e1 | get url | get title

# Waiting (semantic first)
agent-browser wait @e1 | wait --text "Success" | wait --load networkidle

# Profiles & auth state
agent-browser --session mobile open <url> --viewport 375x812
agent-browser state save <path>/auth.json | state load <path>/auth.json
```

When a `browser-use` plugin is available it may be preferred; record which driver ran each high-risk flow, and never silently swap drivers mid-journey.

## Anti-patterns

- **Verifying from the developer seat** — checking the DB, reading the code, or hitting an internal endpoint to decide a step passed. The independent read path must be one the product exposes to its users.
- **Screenshot-everything** — hundreds of images per run bury the ones that matter. Checkpoints and failures.
- **Stale-ref action** — acting on refs from before a DOM change produces phantom results.
- **One desktop window for every persona** — the Mobile persona in a 1280px window finds no mobile bugs.
- **Optimistic-UI trust** — "Saved" on screen with no fresh-load check is the most common false pass.
- **Working around a stall** — a hang, a dead button, an infinite spinner is a *finding*. Record it, file it, move on (per the persona-fidelity guardrails, routed together with this file at Step 3).
