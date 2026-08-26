# CH-agent-comms-operator-fence: Break a live delegation tree and prove Activity still counts and names it truthfully

```yaml
charter:
  id: CH-agent-comms-operator-fence
  mission: "As Ada, break live delegation trees — cancel mid-flight, fail a fan-out, go stale, drain a subtree, restart the daemon — and prove Activity, call detail and the attention bell keep reporting the daemon's own counts and states, with every control mapping to an operation that actually exists."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-supervise-delegation-trees
  scenarios: [RT-delegation-activity-tree, RT-call-record-terminal-states, RT-delegation-attention-signals, RT-session-stop-subtree]
  tour: Interrupt Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Attack the counts first, because they are the load-bearing claim. For every tree, compare the header count on screen against a root_session_id-filtered limit=1 read from the API: they must be equal while far fewer rows are loaded. Fold a tree and confirm its worst state escalates onto the collapsed header rather than hiding. Confirm indent comes from each record's own depth, not from how many ancestors the page happened to load, and that a re-called child renders one subtree rather than two."
      - "Open a call in each of the nine states and confirm detail renders only the controls whose operation exists — cancel in flight, call-again and message once terminal — with nothing greyed in place of absent. Confirm extracted renders as extracted and repaired as repaired, invalid-result keeps both attempts verbatim, completed-without-result says so, a canceled call shows superseded evidence without reopening, and a deadline appears only when one was set. Check the idle-TTL line reads suspended while running and counting while parked."
      - "Take the source stale: confirm the badge contributes zero while rows stay clickable, and that liveness degrading to a poll is stated rather than frozen into a lie. Then act on a detail read that went stale — let the call settle while the panel is open — and confirm the daemon's real current state is reported instead of the action pretending to apply."
      - "Raise attention three ways — an invalid-result, a completed-without-result, and a child blocked on a decision — and confirm each produces one needs-you row under the existing grammar, that a failing fan-out coalesces to one row per tree carrying the real count, that a blocked child appears once as the call row naming its tree rather than also as a bare session row, and that rows clear when their cause resolves with no dismiss, snooze or clear-all anywhere. Confirm no budget-exhausted row exists to find."
      - "Build a three-level tree with one completed result and drain it from the app's stop-subtree control. Try to slip a new call into the tree mid-drain — admission must re-validate the fence and refuse. Kill the daemon partway through and confirm boot resumes from the persisted fence. The report must name stopped children, closed calls and preserved results; the repeat must be idempotent; the completed result must stay fetchable; and the child processes must actually be gone rather than merely marked stopped."
      - "Walk the accessibility floor while the tree is deep and wide: every row reachable by keyboard, every state pairing an icon with a literal word rather than colour alone, and focus surviving a fold and an unfold."
    must_avoid:
      - "Trusting a number rendered next to a list; every count claim in this session is settled by comparing against the daemon's own filtered read."
      - "Judging a row by position in the tree; locate rows by call id, because a live tree reorders."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier — the operator-facing half of the cycle, and the surface where an untruthful UI would
be most expensive because it is where a human decides whether to intervene. It owns invariant 3d
(subtree drain is fence-first: the root-closing fence persists before descendants are enumerated,
admission re-validates it, the drain is idempotent and resumes from the fence on boot) and the
rendering half of invariant 4 (the wake and the record carry exactly the applicable payload, so the
UI has something true to show for every terminal). ADR-001, ADR-003 and ADR-011 are the decisions
visible here — the durable record is what the UI reads, parked is a distinct state whose revival is
calling or messaging rather than an invented Revive button, and there is no budget-exhausted state
because completions are never admission-denied.

The Interrupt Tour is the purpose-built lens: this surface exists for the moment something has
already gone wrong, so the session has to arrive by breaking things rather than by browsing a healthy
tree. The count discipline is the specific regression risk this change carries — task_06 chose
`CallsResponse.total` on an exact-filtered `limit=1` read precisely so a header count could not drift
from the runtime, and a page-length count sneaking back in would be invisible until a tree grew past
one page.

`RT-session-stop-subtree` joins the three web rows because the drain is reachable from this surface
as a control, and because a fence-first guarantee is only meaningful when something is actively being
admitted against it — which is what a live Activity tree provides. Accessibility rides this charter's
`must_try` rather than a separate Sol session: the tree, its folds and its state chips are where the
keyboard and colour-independence floor is actually at risk in this diff.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
