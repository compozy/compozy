# CH-skill-expose-lifecycle-trust: Break every expose halfway through and prove nothing was left behind

```yaml
charter:
  id: CH-skill-expose-lifecycle-trust
  mission: "As Dora, expose and unexpose skills into other tools' folders while killing the operation mid-sequence, occupying paths behind its back, and revoking permissions under it — and prove CompozyOS never modifies an entry it did not create, never leaves a directory or record of its own behind after a failure, and never deletes a skill whose link it could not clean."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-share-skills-with-other-tools
  scenarios: [ET-skill-exposure-lifecycle]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Expose to two targets where the second path is occupied. Exactly one expose_failed envelope must come back — the same shape whether one target failed or several — with the failing target carrying its own code, the completed target marked rolled_back, and only directories this operation created and left empty removed. Then check the filesystem yourself: a preset root the operation created and could not undo is residue, not a detail."
      - "Kill the process between the ownership record and the link. Afterwards the exposure must read missing, never healthy — a record without a live link is the definition of missing — and Expose again must repair it cleanly rather than erroring on the stale record."
      - "Put a foreign symlink at a path CompozyOS owns and re-read: it must report foreign_conflict, offer no repair, and refuse unexpose with expose_foreign_link naming the link's actual target. Ownership is a record whose LinkTarget matches what Readlink returns — confirm filesystem shape alone is never read as ownership by crafting a link that looks like ours but is not."
      - "Walk every refusal to its exact code with nothing written: skill_not_exposable on a bundled skill, profile_skill_not_exposable on a profile-owned skill before any mutation, expose_target_disabled naming the enabled targets, expose_target_invalid for a custom source, expose_name_conflict, expose_link_unsupported, unsafe_skill_name. Try --to compozy and confirm the always-on preset is not an expose target."
      - "Finish on removal ordering: revoke permission on one target folder, then remove the skill. It must refuse with skill_remove_blocked naming the failing link and preserve the canonical directory and the remaining state; restoring permission and re-running must complete cleanup and removal. Then confirm the reverse works — a clean removal deletes the canonical directory only after the last owned link is gone."
    must_avoid:
      - "Repairing state by hand between steps. If a walk needs manual cleanup to continue, that need is the finding — record it before touching anything."
      - "The web Exposures card, its repair affordances, and partial-failure rendering — those belong to CH-skill-expose-web-repair. This session settles the CLI and the wire contract."
      - "Treating an idempotent repeat as a pass without checking the filesystem. Re-expose reporting no change while having quietly replaced a link is exactly the class this session exists to catch."
```

## Selection rationale

Second session of the targeted tier and the second Interrupt Tour, for the same reason as the first:
the invariants here are all about what survives a failure, and none of them can fail during a clean
run. Safety Invariant 3 requires every target to be preflighted before any mutation, the lifecycle
state machines to be followed exactly — expose commits record then link, unexpose removes link then
record, removal deletes the canonical directory only after every owned link is cleaned — and a
mid-sequence failure to roll back completed targets with structured per-target results and no silent
residue. Safety Invariant 4 defines ownership as a record whose path matches and whose `LinkTarget`
equals what `Readlink` returns, and says filesystem shape alone is never ownership; it is what makes
`missing`, `broken`, and `foreign_conflict` three different things rather than three renderings of
"something is wrong". Safety Invariant 5 requires the name to pass validation and the deepest
existing parent to be proven inside the preset root before any write, including creating the preset
root itself.

ADR-011 is the product decision this protects — interop is a symlink, never a copy — and ADR-015
places ownership in a side table with health reconciled from the filesystem, which is precisely why a
crafted look-alike link must not be adopted as ours. The rollback and cleanup-failure paths are the
ones a real operator hits (an occupied path, a read-only folder) and the ones no unit test can prove
end to end, because the evidence is what is left on disk afterwards.

Dora owns it: this is the person who answers for the state of the machine, and the failure that hurts
her most is not an error message but a link CompozyOS created, forgot, and left pointing at nothing —
or worse, one it deleted that belonged to another tool. `ET-skill-exposure-lifecycle` is the single
scenario in scope deliberately: it is dense, every branch needs real filesystem manipulation, and
splitting it across sessions would lose the mid-sequence state that makes the walk meaningful.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
