# CH-agent-comms-spawn-hard-cut-crossings: Go back to the seams spawn used to own and see what fills them

```yaml
charter:
  id: CH-agent-comms-spawn-hard-cut-crossings
  mission: "As Bruno, return to the two behaviors that were defined in terms of the deleted spawn verb — cross-workspace consent and session lineage provenance — and prove the delegation seam now refuses where consent used to negotiate, and that call-created children still read as lineage."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-cross-workspace-access
  scenarios: [ET-workspace-access-mode-matrix, ET-workspace-access-prompt-outcomes, RT-session-parent-provenance]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Re-walk the mode matrix with spawn removed from it: approve-all crossing the native-tool, task-claim and coordination seams promptless, deny-all denying every seam with the exact daemon hint and reason code workspace_access_denied and zero prompts, and approve-reads denying the non-tool seams with the same hint. Confirm exit code 77 on the agent-driven CLI comes from the daemon rather than a local pre-flight block, and that the same decision reads identically over CLI, HTTP and UDS."
      - "Re-walk the approve-reads consent flow with spawn removed from it: one pending permission offering allow_once, allow_session, reject_once and reject_session; once-answers applying to that call only; session answers crossing the task-claim and coordination seams that never prompt; the answer expiring on session stop and on daemon restart; and an unanswered prompt timing out without storing consent."
      - "Then aim at the seam that is no longer part of the funnel. With allow_session cached, point a compozy call and a compozy message at that same foreign workspace: both must be hard-denied with call_workspace_denied before any side effect, and neither may raise a prompt. Repeat under approve-all. A cached session answer must never be laundered into delegation authority — that is the whole point of this session."
      - "Confirm the deleted surfaces do not quietly rejoin the funnel: compozy spawn --workspace, POST /api/agent/spawn, the UDS spawn route and compozy__session_spawn must each respond as genuinely absent rather than as an alias that would route an old crossing back into consent. Confirm nothing in the shipped docs still links to the removed /docs/cli/spawn reference, while the retained internal engine's agents/spawning and autonomy/safe-spawn pages still describe something real."
      - "Re-walk user-session provenance: compozy session new --parent creating a user-type session with parent_session_id, inherited root_session_id and a server-computed spawn_depth, no TTL or auto-stop or budget or narrowing, and deterministic rejections for a missing or cross-workspace parent. Confirm the parent and root filters still work on CLI, HTTP, UDS and the native tool."
      - "Then add the comparison the old walk could not make: create a child through compozy call and confirm it appears in the same --parent and --root lineage filters while still carrying its governance fields, so a provenance-light user session and a governed call child stay distinguishable rather than collapsing into one kind of row."
      - "Read the audit trail through all four readers and confirm they agree on target, seam, source and mode — and that a hard-denied delegation attempt is attributable too, rather than vanishing because it never reached the consent evaluator."
    must_avoid:
      - "Reporting the two scenarios' prior verdicts as still standing; both were reset because a seam they named was deleted, and this session is what replaces that evidence."
      - "Treating the retained internal child-session engine as a surviving public spawn surface."
      - "Leaving lab processes alive; cite teardown.json with clean true."
```

## Selection rationale

Targeted tier. This charter exists because of a reconciliation finding rather than a task's QA-impact
flag: four artifacts in the tree still described behavior in terms of `compozy spawn`, which task_05
hard-cut. Three were scenarios carrying stale verdicts — `ET-workspace-access-mode-matrix` and
`ET-workspace-access-prompt-outcomes` at `blocked-verify`, `RT-session-parent-provenance` at `pass` —
and all three were reset to `untested` under the planner's surface-changed rule, because a verdict
earned by walking a surface that no longer exists is worse than no verdict. The fourth was the
`J-cross-workspace-access` journey itself, repaired in the same pass.

ADR-002 is the decision under test, from the direction the containment charter cannot reach: not
"is the verb gone" but "what happened to the behaviors that were defined in terms of it". The
substantive finding to hunt is the consent boundary. Cross-workspace delegation is not a fifth
consent seam that inherits `allow_session`; it is a hard denial in every permission mode, decided
before any side effect. If a cached session answer could reach a call target, the hard cut would have
quietly widened authority instead of removing a surface.

The Back-Button Tour fits a re-walk of territory that has changed underneath its own documentation:
the session keeps returning to seams that used to behave one way and checking what is actually there
now. Both scenarios are also the tree's only remaining spawn references outside the intentional
negative row, so settling them closes the vocabulary cleanly.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
