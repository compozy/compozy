# J-diagnose-skill-sources — Find out why a skill I expected is not there

A skill the operator is sure they installed does not show up — or shows up as the wrong version.
This journey is the answer path: read the source health, find the one root that is absent,
unreadable, over its cap, or full of links that went nowhere, see which copy won a name collision
and how to reach the loser, fix the cause, and watch the warning clear without restarting anything.

```mermaid
flowchart TD
  A1["Entry: compozy skill sources"] --> B
  A2["Entry: compozy skill sources -o json (or --json)"] --> B
  A3["Entry: compozy skill sources --workspace <ID, name, or path>"] --> B
  A4["Entry: Settings > Skills — open a root's diagnostics"] --> B
  A5["Entry: GET /api/settings/skills over HTTP or UDS"] --> B
  A6["Entry: compozy skill where <name>"] --> W
  B[One read model: effective source order, scope, origin, per-root state and counts]
  B --> C{What is this root's state?}
  C -->|folder is not there| C1["absent — enabled, no count, no error"]
  C -->|permission denied| C2["unreadable — counts omitted entirely, other roots load normally"]
  C -->|over the per-root cap| C3["truncated — real scanned count kept, cap and scanned totals both stated"]
  C -->|entries were skipped| C4["links skipped — one diagnostic per entry naming the path and the reason"]
  C -->|two roots claim one name| C5["collisions — every participating path listed with the usable qualified form"]
  C -->|healthy| C6[Counts plus per-root verification outcome: how many definitions were blocked or warned]
  C4 --> D{Why was the entry skipped?}
  D -->|target deleted| D1[dangling link skipped; the root still finishes scanning]
  D -->|resolves outside every trusted root| D2["escaping link skipped and never followed — including a link reaching into another workspace's or profile's roots"]
  D -->|first-level cycle| D3[cycle detected and skipped; the scan completes]
  D1 --> E
  D2 --> E
  D3 --> E
  C5 --> W["compozy skill where names the winner, every shadowed path, and the qualified form that still reaches the loser"]
  W --> E
  C1 --> E
  C2 --> E
  C3 --> E
  C6 --> E
  E{Same physical directory reached twice?}
  E -->|yes, via a first-level symlink| E1["One catalog entry by realpath, attributed to the highest-precedence root that reached it"]
  E -->|yes, via CompozyOS's own expose link| E2[Never a duplicate and never a self-shadow of its own canonical skill]
  E -->|no| F
  E1 --> F
  E2 --> F
  F["Side effects: skills.scan.truncated and skills.scan.link_skipped appended per scanner pass with root_id, path, reason"]
  F --> G{Does an ecosystem definition make noise?}
  G -->|fields another tool defines| G1["Recognized and silent — no warning, and CompozyOS does not act on them"]
  G -->|genuinely unknown field| G2[Warning still names the field, so the signal survives]
  G1 --> H
  G2 --> H
  H[Operator fixes the cause: creates the folder, repairs permissions, trims the tree, removes the dead link]
  H --> Z["True end: the human table and the JSON agree, the stale warning is gone on the next read without a restart, the skill the operator was looking for is in the catalog, and the copy that lost the collision is still reachable by its qualified form"]
  C2 -.->|operator cannot get permission today| X1["Abandon: the unreadable root stays explicitly unreadable; every other root keeps loading and no count is ever faked as zero"]
  C5 -.->|operator decides the collision is fine| X2["Resume: nothing nags; the qualified form keeps working and the diagnostic stays readable for whoever looks next"]
```

```yaml
journey:
  id: J-diagnose-skill-sources
  name: "Find out why a skill I expected is not there"
  value_statement: "When a skill is missing or the wrong copy won, the product tells me which folder is at fault and why, and the answer stops being a guess."
  personas: [Dora, Bruno]
  entry_points:
    - url: "CLI: compozy skill sources, compozy skill sources -o json, compozy skill sources --workspace <ref>"
      origin: direct
    - url: "CLI: compozy skill where <name>, compozy skill info <name>, compozy skill list --source <tier>"
      origin: direct
    - url: "Web: /settings/skills — per-root diagnostics disclosure"
      origin: in-app-nav
    - url: "HTTP and UDS: GET /api/settings/skills (per-root diagnostic schema)"
      origin: direct
    - url: "Logs: compozy logs --type skills.scan.truncated|skills.scan.link_skipped --component skill"
      origin: direct
  actions:
    - step: 1
      verb: "Read the configured sources and their health in one place"
      expected_observable: "Effective order, scope, origin, per-root existence, counts, truncation, skipped links, and collisions — the same facts in the human table and in the JSON."
    - step: 2
      verb: "Look at a root that is absent, unreadable, or over its scan cap"
      expected_observable: "Absent and unreadable are distinct explicit states; unreadable shows no count at all; truncated keeps the real scanned count beside the cap."
    - step: 3
      verb: "Look at the entries a scan refused to follow"
      expected_observable: "Dangling, escaping, and cyclic first-level links are each skipped with their own diagnostic naming the path and reason, and none of them is fatal to the root."
    - step: 4
      verb: "Ask which copy of a duplicated name won"
      expected_observable: "The winner, every shadowed path, and a qualified form that still reaches the losing copy — with no custom-source slug invented from a path."
    - step: 5
      verb: "Check that a skill installed as a symlink appears once"
      expected_observable: "One catalog entry per physical directory by realpath, attributed to the highest-precedence root that reached it, and CompozyOS's own expose links never shadow their canonical skill."
    - step: 6
      verb: "Load skills written for another tool"
      expected_observable: "Their ecosystem fields load silently and are visibly not honored, while a genuinely unknown field still raises a warning."
    - step: 7
      verb: "Fix the cause and read again"
      expected_observable: "The stale diagnostic is gone on the next read, with no daemon restart."
  goal:
    observable: "The operator can name the root responsible for a missing or shadowed skill, and the surfaces agree on why."
    side_effects: [skills-scan-truncated-event, skills-scan-link-skipped-event, skill-shadowed-event, per-root-verification-summary]
  true_end_state: "The human table and the structured JSON report the same diagnostics for the same roots; the repaired root's warning has cleared without a restart; the skill the operator was hunting is in the catalog; and the copy that lost a collision is still reachable through the qualified form the diagnostic printed."
  exit:
    natural: "The operator stops guessing and goes back to using the skill they were looking for."
  abandonment:
    - at_step: 2
      how: "The operator cannot get permission on an unreadable root today and leaves it broken."
      resume: "The root stays explicitly unreadable, every other root keeps loading, and no count is ever presented as a measured zero."
    - at_step: 4
      how: "The operator decides a name collision is acceptable and does nothing."
      resume: "Nothing nags; the qualified form keeps working and the diagnostic stays readable for whoever looks next."
    - at_step: 7
      how: "The operator trims a huge directory but never comes back to check."
      resume: "The truncation flag clears on its own at the next scan; no manual acknowledgement is needed."
  crosses: [J-absorb-skills-from-other-tools, J-use-absorbed-skills-in-a-session, J-share-skills-with-other-tools, skill-scanner, realpath-containment, VerifyContent, CLI, HTTP, UDS, Web settings, observe-ledger]
```
