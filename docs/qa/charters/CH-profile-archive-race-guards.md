# CH-profile-archive-race-guards: Race archive, create, and delivery against each other until a guard slips

```yaml
charter:
  id: CH-profile-archive-race-guards
  mission: "As Dora, run profile lifecycle mutations concurrently with the things they are supposed to exclude — task claims, automation triggers, session spawns, notification deliveries, pending approvals, a competing create, and an extension install — to prove each guard commits in one transaction and that no interleaving produces work for an archived owner, a duplicated delivery, or two profiles with one name."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-profiles
  scenarios: [ET-profile-lifecycle-race-guards, ET-profile-approval-owner-resume]
  tour: Multi-Tab Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Archive against work creation: with a queued run waiting and a scheduled automation about to fire, archive the owner and prove the claim either completed before the commit or was refused — never a run created for an archived profile. Repeat against a session spawn and a bridge delivery."
      - "Archive against delivery: hold a notification permit open, attempt the archive, and require the retryable `profile_deliveries_in_flight`. Then kill the daemon with the permit row surviving, restart, and count deliveries by delivery id — unarchive must not repeat one."
      - "Archive and delete against approvals: leave an executable pending approval owned by the profile, read both plans, and require it listed as a blocker with `profile_approvals_pending` naming the approval ids; clear it and prove the plans and the mutations change accordingly."
      - "Race the namespace: start a rename so its operation is pending, then attempt to create or rename onto both the reserved old and new names — each must fail `profile_name_taken` naming the holding operation. Fire two same-name creates at once and end with exactly one profile. Run an extension install declaring a name at the same moment the operator creates it and end with one profile, bound not seeded, marked once."
    must_avoid:
      - "Sequential lifecycle correctness and crash recovery (CH-profile-lifecycle-plan-recovery owns those); default COMPOZY_HOME or ports — use the isolated lab from the bootstrap manifest."
```

## Selection rationale

ADR-001 puts the archive state change, the running and leased checks, and the automation pause in
one immediate transaction precisely so an archive cannot race a claim, a trigger, or a spawn.
Safety Invariant 13 keeps `ClaimNextRun` the sole claimer with an owner-active predicate rather than
a second queue; Invariant 18 makes notification dispatch hold a durable owner-active permit through
acknowledgement; Invariant 19 makes pending operations reserve their names and derived paths; and
ADR-002 makes declared-profile creation create-once. Every one of these is a concurrency claim, and
concurrency claims are the ones sequential walks never falsify. The Multi-Tab Tour is the lens: two
actors, one resource, deliberately overlapped.

## Evidence and entry points

- **CLI** — interleaved, timestamped transcripts for each race from two terminals; the archive result payload naming paused automations and frozen queued runs; both reservation refusals.
- **HTTP and UDS** — the plan reads before and after the blocker is cleared, and the refused mutation bodies with their typed codes.
- **Agent** — the pending approval record showing its owner, and the resume result.
- **Runtime** — run and row counts before and after each race; delivery counts by delivery id across the restart; the profile catalog and the create-once marker after the concurrent creates.
- **Web** — none required.

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
