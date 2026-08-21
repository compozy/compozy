# J-operate-profiles — Name a working context and keep its lifecycle honest

An operator gives a working context a name, an identity, and a remembered place, then changes that
context over time — rename, archive, unarchive, delete — through previews that state exactly what
will happen. Every mutation is planned before it is applied, survives a crash, and refuses rather
than half-finishes. Scoped reading, palette projection, layers, extensions, per-profile state, and
the Global view are sibling journeys; this one owns identity, selection, and lifecycle.

```mermaid
flowchart TD
  A1[Entry: menubar switcher] --> B[Read catalog and current selection]
  A2[Entry: Settings → Profiles] --> B
  A3[Entry: CLI compozy profile] --> B
  A4[Entry: HTTP or UDS /api/profiles] --> B
  A5[Entry: agent native profile_list / profile_current] --> B
  B --> C{Which selection wins?}
  C -->|flag or COMPOZY_PROFILE| C1[Explicit selection; nothing sticks]
  C -->|session binding| C2[Session identity is authoritative; a differing flag fails typed]
  C -->|remembered choice| C3[Workspace or Global lens slot restores]
  C -->|nothing remembered| C4[Fall back to default]
  C3 --> D{Remembered profile archived or unavailable?}
  D -->|yes| D1[Fall back to default with the provenance note; explicit selection of it fails]
  D -->|no| E
  D1 --> E
  C1 --> E[Act as the resolved profile]
  C2 --> E
  C4 --> E
  E --> F{Change the catalog?}
  F -->|create| G[Name + identity validated; creation activates the current lens]
  F -->|edit identity| H[Colour and symbol update; archived profiles included]
  F -->|rename / archive / unarchive / delete| P1[Read the plan: exact folders, refs, pauses, removals, blockers]
  G --> S[Side effect: profile row, folder layout, selection row, profile.* event]
  H --> S
  P1 --> P2{Plan still current and profile available?}
  P2 -->|stale revision| R1[Refuse profile_plan_stale; nothing commits; re-read the plan]
  P2 -->|blocked: running sessions, leased runs, delivery permit, pending approval, owned work| R2[Refuse with the typed reason and the safe action]
  P2 -->|remote surface| R3[Refuse profile_remote_management_forbidden; reads still work]
  P2 -->|ok| Q[Apply: one transaction commits rows plus the lifecycle journal]
  R1 --> P1
  R2 --> B
  R3 --> B
  Q --> T[Finalize: journaled filesystem steps run forward-only, each idempotent]
  T --> U{Interrupted before finalize completed?}
  U -->|yes| U1[Profile is unavailable; boot resumes applied and finalizing ops]
  U -->|terminal step failed| U2[Op stays failed and name-reserved until an explicit retry]
  U1 --> V
  U2 --> W[Operator inspects compozy profile ops and retries by id]
  W --> V
  U -->|no| V[Side effect: repo folder renames reported as pending edits the operator commits]
  V --> Z[True end: a fresh read on CLI, HTTP, UDS, native, and Web agrees on the catalog, the selection, and the freed or reserved name; other profiles are byte-stable]
  S --> Z
  G -.->|operator closes the create dialog| X1[Abandon: no profile row, no selection row, no folder]
  P1 -.->|operator walks away from the confirmation| X2[Abandon: plan expires unused; catalog unchanged; reopening re-reads a fresh plan]
```

```yaml
journey:
  id: J-operate-profiles
  name: "Name a working context and keep its lifecycle honest"
  value_statement: "I can name my working contexts, return to the right one automatically, and change or retire one knowing exactly what will happen before it happens — and that a crash cannot leave it half-changed."
  personas: [Ada, Dora, Sol, Bruno]
  entry_points:
    - url: "Menubar profile switcher and Settings → Profiles"
      origin: in-app-nav
    - url: "CLI: compozy profile list|current|use|create|update|rename|archive|unarchive|delete|ops|ops retry"
      origin: direct
    - url: "HTTP and UDS: /api/profiles, /api/profiles/{name}/{rename,archive,unarchive}, DELETE /api/profiles/{name}, /api/profiles/selection, /api/profiles/ops"
      origin: direct
    - url: "Plan reads: /api/profiles/{name}/rename-plan|archive-plan|delete-plan"
      origin: direct
    - url: "Native reads: compozy__profile_list, compozy__profile_current"
      origin: agent
    - url: "Command palette: palette.view.profiles and profile.* actions (handoff only)"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Read which profile I am in and why"
      expected_observable: "Every surface reports the same profile and the same resolution source, and names the fallback when a remembered choice no longer resolves."
    - step: 2
      verb: "Create a profile with a name and an identity"
      expected_observable: "Invalid, taken, and reserved names refuse inline with the rule; a valid one is created, activated for the current lens, and the switcher stops being quiet."
    - step: 3
      verb: "Switch profiles and move between workspaces"
      expected_observable: "The remembered choice updates per workspace and independently for the Global lens; an already-open client is never force-switched."
    - step: 4
      verb: "Read the plan before renaming, archiving, or deleting"
      expected_observable: "The preview names machine folders, repo candidates, dormant placements, ref rewrites, paused automations, frozen queued runs, blockers, and the full removal enumeration."
    - step: 5
      verb: "Apply the planned mutation"
      expected_observable: "The applied result equals the preview field for field, or the mutation refuses with a typed code and a safe next action and changes nothing."
    - step: 6
      verb: "Interrupt a lifecycle operation and come back"
      expected_observable: "The profile reports itself unavailable, boot resumes safe operations forward-only, a terminal failure stays inspectable and name-reserved, and retry does not duplicate committed effects."
    - step: 7
      verb: "Attempt the same mutation from a remote surface"
      expected_observable: "Every profile-state write is refused with profile_remote_management_forbidden while scoped and labeled-aggregate reads keep working."
  goal:
    observable: "The profile catalog, the remembered selection, and the lifecycle journal agree across CLI, HTTP, UDS, native tools, and Web after every change."
    side_effects: [profile-row-written, profile-folder-layout-created, vault-refs-rewritten, selection-row-updated-or-swept, automations-paused, lifecycle-journal-committed, profile-lifecycle-event-emitted, repo-folder-renames-reported-as-pending-edits]
  true_end_state: "A fresh read on every surface shows the same catalog and selection; a deleted name is free and a reserved one names its holder; repo edits are reported as uncommitted working-tree changes, never committed by the daemon; no other profile changed."
  exit:
    natural: "The operator is acting as the intended profile with its remembered choice recorded for this workspace."
  abandonment:
    - at_step: 2
      how: "The operator closes the create dialog or interrupts the CLI before confirming."
      resume: "No profile row, folder, or selection row exists; reopening starts from the daemon's catalog."
    - at_step: 5
      how: "The operator leaves a rename, archive, or delete confirmation open and does something else."
      resume: "The plan is never applied; reopening re-reads a fresh plan, and an outdated revision is refused rather than executed."
    - at_step: 6
      how: "The machine dies between apply and finalize."
      resume: "The profile is unavailable until boot reconciliation resumes the journaled steps, or until the operator retries a failed operation by id."
  crosses: [J-scope-work-by-profile, J-command-profiles-from-palette, J-layer-profile-resources, J-adopt-extension-profiles, J-restore-per-profile-state, J-scope-global-across-workspaces, profile-store, selection-routes, lifecycle-journal, vault, CLI, HTTP, UDS, native-tools, Web]
```
