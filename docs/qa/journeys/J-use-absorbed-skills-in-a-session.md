# J-use-absorbed-skills-in-a-session — Use an absorbed skill in a conversation without paying for it twice

Someone opens a session and types `/`. Skills absorbed from another tool's folder are in the list
next to Compozy's own, each carrying a small label saying where it came from. If the session's
provider already reads that folder itself, CompozyOS quietly stops re-sending the same text into the
prompt — but the skill stays listed, stays selectable, and still arrives in full when the person asks
for it by name.

```mermaid
flowchart TD
  A1[Entry: open a session and type / in the composer] --> B
  A2["Entry: compozy session commands <session-id> -o json"] --> B
  A3["Entry: GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands"] --> B
  A4[Entry: session startup prompt assembly] --> S
  B[Command catalog: built-ins, agent commands, and skills from every enabled source]
  B --> B1["Absorbed rows carry a discreet origin label; Compozy-native rows carry none and are otherwise unchanged"]
  B1 --> B2{Two enabled sources carry the same bare name?}
  B2 -->|yes| B3["Both stay reachable and distinguishable; each has a stable qualified form the picker shows"]
  B2 -->|no| C
  B3 --> C
  S{Does this session's provider read the winning root natively?}
  S -->|provider is claude and the skill won from a claude-native folder| S1["Omitted from prompt sections A and B only"]
  S -->|provider is openclaw or hermes and the folder matches at that exact level| S2["Omitted at per-folder granularity — a provider that reads only the workspace-level folder does not suppress the global one"]
  S -->|the skill won from a Compozy root instead| S3[Injected normally — suppression keys on the winning root, never on the name]
  S -->|provider home relocated or isolated| S4[The operator-home folders are no longer native to this session, so those skills are injected normally]
  S -->|provider unknown or unset| S5[Nothing is suppressed — fail open to inclusion]
  S -->|no enabled source is native to this provider| S5
  S1 --> T
  S2 --> T
  S3 --> C
  S4 --> C
  S5 --> C
  T["Every suppression decision is observable in harness diagnostics with its reason — and stays a log, never a durable event"]
  T --> C[The skill is still listed, still enable/disable-able, still readable through the settings and skill APIs]
  C --> D[Person picks a skill and submits the turn]
  D --> E{Is the pick still valid at submit time?}
  E -->|source disabled between picking and submitting| E1["Source-drift rejection — deterministic, nothing partial injected, transcript not mutated"]
  E -->|content fails verification| E2[Turn rejected with the existing deterministic error; nothing partial is injected]
  E -->|body over the per-skill budget| E3[Truncated by the existing invocation budget rules, exactly as a native skill would be]
  E -->|valid| F["Content delivered into the turn — including for a skill that was suppressed from the prompt, because an explicit request wins"]
  E1 --> Z
  E2 --> Z
  E3 --> F
  F --> G{Something changes mid-session}
  G -->|a source is enabled| G1[Its skills appear in the picker on the next catalog refresh without recreating the session]
  G -->|a source is disabled| G2["Its rows leave the picker on the next refresh; content already delivered in this conversation is unaffected"]
  G -->|the remembered profile is switched| G3["This session keeps the profile catalog it was created with; the switch does not leak into it"]
  G1 --> Z
  G2 --> Z
  G3 --> Z
  Z["True end: the person got the skill they asked for exactly once, the prompt carried no duplicate copy of what the provider already had, the picker and the structured catalog agree on the same rows and origins, and every omission is explainable from diagnostics rather than invisible"]
  D -.->|person picks a skill then abandons the turn| X1["Abandon: nothing is injected and nothing is recorded; the composer draft is all that remains"]
  G2 -.->|person keeps the tab open for hours after the source is gone| X2["Resume: the next refresh is truthful; the stale row does not linger as an invocable command"]
```

```yaml
journey:
  id: J-use-absorbed-skills-in-a-session
  name: "Use an absorbed skill in a conversation without paying for it twice"
  value_statement: "Skills from any enabled folder are usable in a session exactly like Compozy's own, and the prompt never carries a second copy of what the provider already reads for itself."
  personas: [Théo, Bruno, Ada]
  entry_points:
    - url: "Web: session composer / picker"
      origin: in-app-nav
    - url: "Session: startup prompt assembly and per-turn prompt input"
      origin: direct
    - url: "CLI: compozy session commands <session-id> -o json"
      origin: direct
    - url: "HTTP and UDS: GET /api/workspaces/{workspace_id}/sessions/{session_id}/commands"
      origin: direct
    - url: "Diagnostics: harness suppression diagnostics (skills.injection.suppressed log records)"
      origin: direct
  actions:
    - step: 1
      verb: "Type / in a session that has skills from several sources"
      expected_observable: "Absorbed skills appear beside native ones, each foreign row carrying a discreet origin label and native rows carrying none."
    - step: 2
      verb: "Start sessions under each supported provider and its aliases"
      expected_observable: "Only skills whose winning root that provider already reads natively are omitted from the prompt, at per-folder granularity, and an unknown provider suppresses nothing."
    - step: 3
      verb: "Ask why a skill is not in the prompt"
      expected_observable: "Harness diagnostics name the skill, its source, the provider, and the reason — the omission is explainable, not invisible."
    - step: 4
      verb: "Explicitly invoke a skill that was suppressed from the prompt"
      expected_observable: "Its content is delivered into the turn in full; the explicit request wins over the suppression."
    - step: 5
      verb: "Invoke two same-named skills from different roots through their qualified forms"
      expected_observable: "Each qualified form reaches its own physical skill deterministically, and the form stays stable for a given configuration generation."
    - step: 6
      verb: "Switch the remembered profile, then disable a source between picking a skill and submitting"
      expected_observable: "The existing session keeps its original profile catalog, and the stale pick is rejected as source drift with nothing partial injected and no transcript mutation."
  goal:
    observable: "The person invokes any absorbed skill as easily as a native one, and the prompt contains each skill's text at most once."
    side_effects: [prompt-sections-a-and-b-filtered, skills-injection-suppressed-log-record, session-command-revision-broadcast, skill-invocation-recorded-in-transcript]
  true_end_state: "The requested skill arrived exactly once, the prompt carried no duplicate of what the provider already reads, the web picker and the structured command catalog list the same rows with the same origins and the same revision, and every omission is traceable in diagnostics."
  exit:
    natural: "The conversation continues with the skill's content in play and no wasted context."
  abandonment:
    - at_step: 4
      how: "The person picks a skill from the menu and then abandons the turn without submitting."
      resume: "Nothing is injected and nothing is recorded; only the composer draft remains."
    - at_step: 6
      how: "The person leaves the tab open for hours after an operator disables the source."
      resume: "The next catalog refresh is truthful; the stale row does not linger as an invocable command, and content already delivered earlier in the conversation is untouched."
  crosses: [J-absorb-skills-from-other-tools, J-use-session-slash-commands, J-load-skill-in-managed-session, session-harness, injection-policy, command-catalog-projection, provider-env-and-home, Web composer, CLI, HTTP, UDS]
```
