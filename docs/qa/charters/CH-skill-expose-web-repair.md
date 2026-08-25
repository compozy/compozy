# CH-skill-expose-web-repair: Navigate away from every expose action and come back to the truth

```yaml
charter:
  id: CH-skill-expose-web-repair
  mission: "As Bruno, expose and repair skills from the Marketplace detail while pressing back at every step, deep-linking mid-flow, and reloading after each action — and prove the page always shows the link's real reconciled state, offers repair only for links CompozyOS made, and never re-fires an expose because of a navigation."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-share-skills-with-other-tools
  scenarios: [ET-web-skill-expose-panel, ET-web-marketplace-installed-management, ET-web-marketplace-skill-install]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Open the target picker, press back, and come forward again — the multi-select must not hold a stale selection and must still list only enabled presets. Fire an expose, press back during the pending state, then return: the operation must not run twice, and the row must show whatever actually happened rather than the optimistic state the page left with."
      - "Walk the four statuses and their affordances exactly: healthy shows the link path; a link deleted on disk reads as deleted with repair actions and Expose again restores it; an unresolvable link of ours reads as broken with repair allowed; a foreign entry reads as another app's file with no action at all. Reload after each and confirm the status survives a fresh load rather than living in page state."
      - "Expose to two targets where one path is occupied and confirm the failure names both targets, marks the compensated one rolled back, and quotes the daemon's per-target codes verbatim. Then press back from that failure and return — the results must still be readable, not replaced by an empty card."
      - "Confirm a bundled skill shows no Exposures card at all — absent, not disabled — and that the installed rows carry the neutral mono origin pill only for skills absorbed from a non-Compozy source. Install a skill whose display name, entry id, and install slug all differ, confirm the card reflects the new installed state, and confirm Manage still routes to the installed skill."
      - "Bookmark a mid-flow detail URL, open it in a second tab after acting in the first, and confirm both tabs converge on the same reconciled state. Under a named profile, confirm the kind page's skill list stays scoped to that exact profile rather than following the remembered one."
    must_avoid:
      - "Offering or accepting any action on a foreign_conflict row. CompozyOS never touches an entry it did not create; a repair affordance rendered there at all is the finding, whether or not clicking it does anything."
      - "CLI and wire-level expose semantics — rollback ordering, ownership authority, removal blocking belong to CH-skill-expose-lifecycle-trust."
      - "Asserting on copy where a stable selector exists: skill-exposures-card, skill-expose-panel and its row, result, and failure children, and the skill-expose-target-picker family."
```

## Selection rationale

The Back-Button Tour, because this surface is a mutation flow living inside a routed detail page —
picker opens, operation fires, status reconciles — and the whole class of bugs it hides is navigational:
a stale selection restored on forward, an expose re-fired by a back-then-submit, a status that lives
in page state and evaporates on reload, a failure card replaced by an empty one when the user returns
to read it. None of those appear in a straight-line walk.

ADR-011 and ADR-015 supply the rules the page has to render truthfully rather than derive: ownership
is a record plus a matching link target, so `missing`, `broken`, and `foreign_conflict` are three
distinct facts, and the `_uiux.md` S3 contract binds each to a distinct affordance set — repair
actions for the two that are ours, and *zero* affordances for the one that is not. That last one is
the session's sharpest assertion, because rendering a disabled repair button on a foreign entry would
already imply Compozy might touch it.

The two marketplace scenarios ride along because this cycle reset them and they share the exact
surface: the installed card gained the origin pill, the installed detail gained the Exposures card
beside content, shadows, enable, and update, and the kind page's skill query moved to the exact
profile. Walking them in a separate session would mean loading the same page twice for the same diff.
This is deliberately *not* the cycle's canary — marketplace acquisition here is collateral on a
surface the diff genuinely changed, whereas the canary must be a journey the diff was never supposed
to touch.

Bruno owns it rather than Dora: the person exposing a skill from the web is the builder who wrote it
and wants another tool to see it, not the administrator who set the source policy.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
