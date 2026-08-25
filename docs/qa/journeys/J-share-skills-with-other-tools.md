# J-share-skills-with-other-tools — Let my other tools read a skill I keep in Compozy

A skill author keeps one canonical copy in Compozy and wants Claude Code or an `agents`-convention
tool to read it too. CompozyOS puts a link in that tool's folder — never a copy — and stays honest
about that link forever: it repairs the ones it made, refuses to touch anything it did not make,
rolls back cleanly when one target of a multi-target request fails, and will not delete a skill
while one of its own links is still stuck.

```mermaid
flowchart TD
  A1["Entry: compozy skill create <name> --expose agents,claude"] --> B
  A2["Entry: compozy skill expose <name> --to agents,claude"] --> B
  A3["Entry: Marketplace > Skills > installed detail > Exposures card"] --> B
  A4["Entry: POST /api/skills/{name}/expose over HTTP or UDS"] --> B
  B{Is this skill allowed to be exposed?}
  B -->|bundled, no on-disk home| B1["Refused skill_not_exposable — points at compozy skill create to make a copy first; no Exposures card is offered on the web at all"]
  B -->|owned by a profile or workspace-profile| B2["Refused profile_skill_not_exposable before any mutation; no shared provider-root entry is created"]
  B -->|eligible user or workspace skill| C{Preflight every target}
  B1 --> Y
  B2 --> Y
  C -->|target is not an enabled preset| C1["expose_target_disabled — names the disabled source and the targets that are enabled"]
  C -->|target names a custom source| C2["expose_target_invalid — expose targets are presets; custom sources cannot receive links"]
  C -->|name is unsafe as a path segment| C3[unsafe_skill_name — names the offending characters; nothing written]
  C -->|something already occupies the path| C4[expose_name_conflict — names the occupying path; nothing written]
  C -->|filesystem cannot link| C5[expose_link_unsupported — deterministic; never a silent copy fallback]
  C -->|all targets clear| D["Commit per target: record first, then link. An enabled preset whose folder does not exist yet is created, containment-proven, and added to the rollback set"]
  C1 --> E
  C2 --> E
  C3 --> E
  C4 --> E
  C5 --> E
  E{Did any target fail?}
  E -->|yes| E1["One envelope: expose_failed with per-target results; already-completed targets are compensated and marked rolled_back"]
  E1 --> E2{Could a compensation be undone?}
  E2 -->|no| E3[Per-target cleanup error surfaced — no silent residue]
  E2 -->|yes| E4["rolled_back: true; only folders this operation created and left empty are removed"]
  E3 --> F
  E4 --> F
  E -->|no| D1["Side effect: skills.exposure.created per target with skill, target, link_path"]
  D1 --> F[Inspect the link's health]
  F --> G{What is at the path now?}
  G -->|our link, resolves| G1[healthy — the external tool reads the canonical body through it]
  G -->|record with nothing at the path| G2["missing — Expose again repairs it"]
  G -->|our link, no longer resolves| G3["broken — unexpose or expose again are both allowed"]
  G -->|something that is not our link| G4["foreign_conflict — reported and never touched; the web offers no action at all, and unexpose refuses with expose_foreign_link"]
  G1 --> H
  G2 --> H
  G3 --> H
  G4 --> H
  H{Next lifecycle event}
  H -->|expose the same target again| H1[Idempotent: already exposed, no change, not an error]
  H -->|unexpose| H2["Link removed, record removed, skills.exposure.removed; running it twice converges to not exposed"]
  H -->|marketplace update| H3[The skill's path is preserved, so existing links stay valid]
  H -->|remove the skill| H4{Every owned link cleaned?}
  H4 -->|yes| H5[Canonical directory deleted only after the last owned link is gone]
  H4 -->|no| H6["skill_remove_blocked — names the failing link; the skill and its state are preserved; retry after fixing the cause"]
  H1 --> Z
  H2 --> Z
  H3 --> Z
  H5 --> Z
  H6 --> Z
  Y --> Z
  Z["True end: the other tool opens the link and gets the canonical body; CLI, GET /api/skills/{name}, compozy__skill_view, and the web Exposures card report the same reconciled status for the same link; no foreign entry was ever modified; and nothing CompozyOS created was left behind"]
  D -.->|crash between the record and the link| X1["Resume: the exposure reads as missing afterwards, never as healthy; Expose again repairs it cleanly"]
  C4 -.->|author walks away after the conflict| X2["Abandon: the skill still exists and is unexposed; no half-created link is ever visible"]
```

```yaml
journey:
  id: J-share-skills-with-other-tools
  name: "Let my other tools read a skill I keep in Compozy"
  value_statement: "One canonical skill lives with Compozy and other tools read it through a link Compozy maintains honestly — repairing what it made, never touching what it didn't, and never rotting."
  personas: [Dora, Bruno, Ada]
  entry_points:
    - url: "CLI: compozy skill create <name> --expose <targets>, compozy skill expose <name> --to <targets>, compozy skill unexpose <name> --to <targets>"
      origin: direct
    - url: "CLI: compozy skill info <name>, compozy skill where <name>"
      origin: direct
    - url: "Web: /marketplace/skills, /marketplace/skills/$entryId — installed detail Exposures card"
      origin: in-app-nav
    - url: "HTTP and UDS: POST /api/skills/{name}/expose, POST /api/skills/{name}/unexpose, GET /api/skills/{name}"
      origin: direct
    - url: "Native: compozy__skill_view exposures[]"
      origin: agent
  actions:
    - step: 1
      verb: "Create a skill and expose it to another tool's folder in one step"
      expected_observable: "The canonical skill exists in the Compozy folder, a link exists at the target path, and the external tool resolves that link to the canonical content."
    - step: 2
      verb: "Try to expose something that is not allowed to be exposed"
      expected_observable: "A bundled skill is refused with the copy-first guidance and shows no exposure affordance on the web at all; a profile-owned skill is refused before any mutation and creates no shared entry."
    - step: 3
      verb: "Expose to two targets where one path is already occupied"
      expected_observable: "One expose_failed envelope naming both targets, the failing target carrying its own code, the completed target marked rolled back, and only folders this operation created and left empty removed."
    - step: 4
      verb: "Damage a link outside CompozyOS and look at it again"
      expected_observable: "A deleted link reads missing with repair actions; an unresolvable link of ours reads broken with repair actions; a foreign entry reads as a conflict with no action offered anywhere."
    - step: 5
      verb: "Repeat expose and unexpose on the same target"
      expected_observable: "Repeating either converges without error — expose is idempotent, unexpose runs twice to the same not-exposed end state."
    - step: 6
      verb: "Update and then remove the skill"
      expected_observable: "Update preserves the path so links stay valid; removal deletes the canonical directory only after every owned link is cleaned, and an uncleanable link blocks removal with a retryable named error instead of leaving residue."
  goal:
    observable: "Another tool reads the canonical skill body through a link CompozyOS created, and every inspection surface reports that link's true reconciled state."
    side_effects: [skill-exposure-record-written, symlink-created-in-target-root, preset-root-directory-created, skills-exposure-created-event, skills-exposure-removed-event, skills-exposure-operation-failed-event, skills-exposure-broken-detected-event, skills-exposure-cleanup-failed-event]
  true_end_state: "The external tool opens the link and gets the canonical body; the CLI, GET /api/skills/{name}, compozy__skill_view, and the web Exposures card all report the same reconciled status for the same link; no entry CompozyOS did not create was modified; and no directory or record CompozyOS created was left behind after a failure."
  exit:
    natural: "The author keeps editing one canonical skill and the other tool keeps seeing the current version."
  abandonment:
    - at_step: 1
      how: "The process is interrupted between writing the ownership record and creating the link."
      resume: "The exposure reads as missing afterwards, never as healthy; Expose again repairs it cleanly."
    - at_step: 3
      how: "The author hits a name conflict and gives up on exposing."
      resume: "The skill still exists and is simply not exposed; no half-created link is ever visible and nothing must be cleaned by hand."
    - at_step: 6
      how: "Removal is blocked by a link the operator cannot clean right now."
      resume: "The skill and its remaining state are preserved; re-running removal after fixing the cause completes cleanup."
  crosses: [J-absorb-skills-from-other-tools, J-diagnose-skill-sources, J-marketplace-acquisition, skill-exposures-store, expose-manager, filesystem-containment, CLI, HTTP, UDS, Web marketplace detail, native-tools]
```
