# CH-loop-legibility-tasks-calm-web: Prove the reveal stays ephemeral and provenance survives a deep link

```yaml
charter:
  id: CH-loop-legibility-tasks-calm-web
  mission: "As Dora, navigate in and out of Tasks with the loop reveal on, deep-link straight into a loop execution record, and prove the reveal never persists into the URL or a return visit while the record's own page always names its run and links back."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-loop-steady-state
  scenarios: [TA-web-tasks-calm-default-reveal, TA-web-task-detail-loop-provenance]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Turn the reveal on, open a record, press back, then navigate away and return: the reveal must be off every time. Confirm it is nowhere in the URL and nowhere in config — it is component state by design, so a reveal that survives a reload is the finding."
      - "Switch List / Kanban / Dashboard with the reveal on and off and confirm the board is the same population as the list and that group counts match their rows in both states."
      - "Deep-link to /tasks/<record-id> with a cold cache, never arriving from the list: the properties rail must lead with the Loop run or Loop step block from the single-task read alone, with a working Open run link on a terminal run as well as a live one, and no field on either page recovered by parsing a task id — the old loop.<run>.g<N>.node.<id>.<item> parsing and its nesting machinery are deleted, so an absent field is omitted rather than reconstructed."
      - "Walk the three degraded reads and confirm each says something different: the reveal-scoped empty names the filter and offers the way out, the true empty says there is no work yet, and a record whose run retention deleted keeps its run id and reads Run no longer available with no link — that degrade must come from the absent loop name, not from probing the run link."
    must_avoid:
      - "Re-testing the retired nesting behavior — TA-web-task-list-loop-subtask-nesting is superseded by the exclusion contract; there is no coordinator row to nest under."
      - "Judging the reveal from a single page load; the assertion is what happens on the way back."
```

## Selection rationale

Targeted tier — the web half of the calm-default hot spot. Owns Safety Invariant 8 (one server-side
filtered set behind every surface) and ADR-001's exclusion-with-typed-reveal contract, plus the
US-002 acceptance that the reveal is quiet and ephemeral. The Back-Button Tour is the exact lens:
every claim in this pair is about navigation state — the reveal must not persist, and the record's
page must not depend on having arrived from a list. Both are invisible to a single-page check and
obvious the moment someone presses back. This is also where the delete targets get verified —
`task-hierarchy.ts` and `task-subtask-list.tsx` are gone, and nothing may have quietly survived
them.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
