# CH-profile-aggregate-owner-truth: Read the aggregate as someone who has never heard the word "scope"

```yaml
charter:
  id: CH-profile-aggregate-owner-truth
  mission: "As Cora, turn on All profiles and try to answer four plain questions on every screen — whose is this, where will this go if I make it, why am I seeing something from somewhere else, and why is this list empty — without knowing any runtime vocabulary, and flag every place the screen answers in jargon, in colour alone, or not at all."
  mode: charter-with-tour
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-scope-work-by-profile
  scenarios: [ET-profile-aggregate-chip-toast, ET-profile-web-aggregate-owner-surfaces]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "With three profiles, one of them archived, turn All profiles on and read every listing — sessions, tasks, loop runs, automation jobs and triggers, worktrees. Each row must say whose it is in words, and the archived owner must read as archived rather than merely being dimmer. Switch to one profile and confirm the tags disappear entirely — a tag on every row in a scoped view is noise, not truth."
      - "Open every shared creation surface while aggregated: the destination chip must be visible without hovering, be plain text with no control attached, and say where the item will go before you commit. Create one on each and confirm the success message names the same owner and the row then carries it. Confirm no surface offers a way to change the destination from inside itself."
      - "Paste a link to something owned by another profile: it must not bounce you, the banner must name the owner in plain words, the switch must land you there, and the lists around it must not have widened. Then enter a profile with no work and confirm each empty listing names that profile instead of only saying the list is empty."
      - "Read the Home usage panel scoped and aggregated: scoped shows only this profile with no breakdown offered; aggregated adds a per-profile breakdown including the archived owner and pre-profile history under `default`. Judge whether a person who does not know the word `default` can tell what that row means."
    must_avoid:
      - "Structured surfaces — no CLI, no API, no reading a payload to decide whether a screen is honest (CH-profile-foreign-leak-probe owns those); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest and a production-parity web build."
```

## Selection rationale

The `_uiux.md` constraints are product promises, not styling notes: identity colour is user data and
never a signal, colour never carries meaning alone, the destination chip is text rather than colour,
and the UI must be truthful about what the runtime supports. ADR-005 chose a fixed create-in-default
chip over a destination picker specifically so the operator is told rather than asked — which only
works if the telling is legible. Safety Invariant 2 says the aggregate is always owner-labeled.
Cora is the roster's least technical persona and the only lens that catches an answer that is
technically present but unreadable; running her against the aggregate is how the plain-language read
gets tested rather than assumed.

## Evidence and entry points

- **Web** — screenshots of the aggregate and scoped listings side by side, the archived owner tag, a worktree row in two profiles, the destination chip on each selector-bearing creation surface, the owner toast, the deep-link banner before and after the switch, one profile-named empty state, and both usage states.
- **Plain-language notes** — for every screen, the four questions and whether they were answerable without runtime vocabulary; every phrase Cora had to guess at is a finding even when the behavior is correct.
- **CLI, HTTP, UDS, agent** — deliberately none; this session judges the surface, and its verdicts must not lean on a payload the persona cannot see.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
