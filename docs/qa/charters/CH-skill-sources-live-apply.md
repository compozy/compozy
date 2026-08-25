# CH-skill-sources-live-apply: Change what Compozy scans while it is running and never catch it lying

```yaml
charter:
  id: CH-skill-sources-live-apply
  mission: "As Dora, turn source folders on and off, add and remove team directories, and override one workspace — interrupting and overlapping the writes on purpose — and prove that every surface converges on exactly the policy that was last committed, with no restart, no half-written file, and no count that was never measured."
  mode: charter-with-tour
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-absorb-skills-from-other-tools
  scenarios: [ET-manage-skill-source-policy, ET-live-skill-source-reload, ET-skill-origin-attribution]
  tour: Interrupt Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Overlap two writes to the same scope on purpose and land them close enough that one is still in flight when the other commits. The later generation must win everywhere, the earlier one must report as superseded, and no read surface may ever serve the discarded generation — check the catalog, the settings envelope, and the session command revision, not just the CLI response."
      - "Interrupt writes mid-flight: kill the CLI between validation and commit, close the settings page mid-draft, cancel a PATCH. Each must leave no partial config file and no apply record, and the previous value must still resolve on a cold read."
      - "Toggle a preset rapidly several times and stop. The last saved state must win with no residual skills from an intermediate state, and the count on every surface must match a fresh scan rather than a cached one."
      - "Walk each validation refusal and confirm nothing was applied: unknown_skill_source with its valid list and closest match, duplicate_skill_source naming the source that already owns the resolved path, invalid_source_path for a workspace-relative path at user scope, workspace_scope_field_forbidden for a non-source field. Then re-read from a fresh process — an optimistic response that a cold read contradicts is the finding."
      - "Set a workspace override on one key only, confirm the other key still says it is inheriting, return the first to inherited, and verify a second workspace through the API was never touched. Then read origin attribution end to end: the CLI ORIGIN column, GET /api/skills, the native tools, and the extension Host API skills/list must name the same winning source, and a Compozy-native skill must carry an explicit empty origin rather than an invented label."
    must_avoid:
      - "Restarting the daemon to make a change appear. A restart that fixes the symptom is itself the finding — this feature's whole claim is live apply."
      - "Accepting the response body as the verdict. Every accepted mutation is settled by a cold re-read from a fresh process or request, never by the optimistic payload."
      - "Drifting into per-root scan diagnostics — truncation, unreadable roots, skipped links, collisions belong to CH-skill-sources-diagnostics-truth and a finding there goes to that charter's follow-up."
```

## Selection rationale

First session of the targeted tier, because the generation fence is this cycle's highest-blast-radius
invariant and the only one that cannot be found by walking calmly. Safety Invariants 8 and 9 say root
resolution is atomic per generation and that every asynchronous commit point — registry swap,
resource publication, diagnostics replacement, session-command broadcast — re-compares the generation
immediately before committing, so generation N can never overwrite N+1 on any read surface. Safety
Invariant 12 says the skill cache is keyed by stable profile id, applicable workspace id, and the
effective four-layer source generation, so no two owners share a projection. Nothing about either
invariant fails when writes are serialized by a careful tester; both fail only under overlap and
interruption, which is exactly what the Interrupt Tour forces.

ADR-005 (source configuration applies live, no daemon restart) is the product promise sitting on top
of those invariants, and ADR-006 (workspace override ships with full CLI, API, and web management)
plus ADR-007 (replace-on-present merge with root lists) are what the per-key inheritance walk
settles. ADR-013 (origin is visible where skills are chosen and managed) is why origin attribution
rides in this session rather than waiting: the attribution is computed from the same generation the
fence protects, so a fence bug and an attribution bug look identical from the outside and are
cheapest to separate in one sitting.

Dora owns this because the failure mode is an operator who changed a policy, was told it applied, and
is looking at a surface that disagrees — the trust-damage class she is defined to reveal.
`ET-manage-skill-source-policy` and `ET-live-skill-source-reload` were flagged `untested` by tasks 01
and 02 and have never been walked; `ET-skill-origin-attribution` was flagged by task 04 and extended
by the late CLI profile-tier fix, so its `--source` filter half is newer than anything that has been
tested.

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
