# CH-profile-lifecycle-plan-recovery: Interrupt every lifecycle mutation and prove the preview was the truth

```yaml
charter:
  id: CH-profile-lifecycle-plan-recovery
  mission: "As Ada, drive the full profile lifecycle through CLI, HTTP, and UDS while interrupting it at every seam — between plan and apply, between apply and finalize, and at a terminal step failure — to prove the applied result equals the preview field for field, that a stale plan never commits, that recovery is forward-only and never duplicates, and that a failed operation stays inspectable and reserved until an explicit retry."
  mode: charter-with-tour
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-profiles
  scenarios: [ET-profile-cli-lifecycle, ET-profile-operations-recovery, ET-profile-selection-precedence, ET-profile-remote-write-boundary]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Read each plan, then change the profile from another terminal before submitting: rename, archive, and delete must each fail `profile_plan_stale` with nothing committed, and re-reading must produce a plan that applies cleanly. Then apply each plan unmodified and diff the result against the preview field for field — the delete removal enumeration especially."
      - "Interrupt between apply and finalize on rename, archive, and delete; restart the daemon; prove the profile reports itself unavailable, that boot resumes only `applied` and `finalizing` operations, and that a `failed` one does not restart itself. Retry it by id and prove committed effects are not duplicated."
      - "Walk the resolution chain against a moving target: set different workspace and Global remembered choices, override with `COMPOZY_PROFILE` and `--profile`, archive the remembered profile mid-flight and require the fallback note, and prove `daemon`, `doctor`, and `update` ignore all of it even with an invalid value set."
      - "Prove `default` is permanent (rename, archive, and delete all refuse) and that a rename leaves selections untouched while rewriting machine folders and vault refs — then attempt every mutation from a paired remote surface and require `403 profile_remote_management_forbidden` with no state change."
    must_avoid:
      - "Concurrency (CH-profile-archive-race-guards owns races and reservations); the Settings dialogs (CH-profile-settings-dialog-plans owns those); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest."
```

## Selection rationale

Safety Invariants 14, 15, 16, and 17 stake the whole lifecycle on one claim: row state and the
operation journal commit atomically after plan-revision validation, pre-commit failures touch no
filesystem, and post-commit recovery is forward-only. Invariant 7 makes the removal catalog total so
preview equals applied. ADR-012 adds the rename contract — daemon-owned name-bearing artifacts
rewrite, id-keyed selections do not — and ADR-001 makes `default` permanent. None of this is
observable on a happy path; it is only observable when the walk stops the machine in the middle. The
Interrupt Tour exists for exactly that.

## Evidence and entry points

- **CLI** — structured transcripts for every verb in all four output formats; the plan beside the applied result for rename, archive, and delete; the machine-command parity output under an invalid `COMPOZY_PROFILE`.
- **HTTP and UDS** — the three plan payloads, the mutations quoting their revisions, the `409 profile_plan_stale` bodies, and the `403` remote refusals on both listeners.
- **Runtime** — the fault-injection transcript, pre- and post-restart operation payloads (id, kind, profile, status, step, error), the `profile.lifecycle_op_recovered` and `_failed` events, side-effect counts across the retry, and proof that no error or event carries a secret value or vault reference.
- **Agent** — `compozy__profile_list` and `compozy__profile_current` read from inside a session, confirming the session source and immutable binding.
- **Web** — none required.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
