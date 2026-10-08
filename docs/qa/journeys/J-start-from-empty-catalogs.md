# J-start-from-empty-catalogs — Start real work from zero inventory

A workspace with no Tasks or Automations should teach each object, expose only
real next actions, and remain truthful after an action or cancellation.
Automations replaced the separate Jobs and Triggers catalogs in v0.4.0.

```mermaid
flowchart TD
    E[Entry: empty Tasks or Automations catalog] --> K{Catalog}
    K -->|Tasks| T[Read intro and review a collapsed template]
    T --> TE[Open the existing editor with that template]
    TE --> TC[Cancel and return to the unchanged empty catalog]
    K -->|Automations, suggestion| J[Read intro and review Suggested automations]
    J --> JA[Create one and dismiss another]
    JA --> JR[Refresh: created automation remains and resolved proposals stay gone]
    K -->|Automations, event start| G[Choose When something happens]
    G --> GE[Editor opens with Starts preselected]
    GE --> GC[Cancel and return with no automation created]
    TC --> Z[True end: every catalog remains truthful on fresh load]
    JR --> Z
    GC --> Z
    E -.->|apply an unmatched filter| F[Abandon: filtered-empty offers Clear filters]
    F -.->|clear filters| E
```

```yaml
journey:
  id: J-start-from-empty-catalogs
  name: "Start real work from zero inventory"
  value_statement: "An operator can understand an empty catalog and reach a real next action without sample data or unsupported controls."
  personas: [Bruno, Cora]
  entry_points:
    - url: "web /tasks, /automations"
      origin: in-app-nav
  actions:
    - step: 1
      verb: "Open each empty catalog from normal navigation"
      expected_observable: "The intro names the object and one real next action; Automations may show live workspace suggestions (schedules only)"
    - step: 2
      verb: "Review a Task template and an automation suggestion"
      expected_observable: "Details disclose on demand and actions remain outside the disclosure control"
    - step: 3
      verb: "Open the Task create flow and the automation editor from both empty-state starts, and resolve suggestions"
      expected_observable: "Existing editors open; one scheduled automation (job) is created, one proposal is dismissed, and no UI-only record appears"
    - step: 4
      verb: "Refresh and revisit filtered-empty states"
      expected_observable: "Server-owned changes persist, cancellations leave no residue, and Clear filters restores the unfiltered state"
  goal:
    observable: "Fresh loads agree with the actions taken and never present unsupported suggestions"
    side_effects: [automation-job-created, automation-suggestion-dismissed]
  true_end_state: "A refresh shows the accepted automation, keeps the dismissed proposal gone, and leaves cancelled Task and automation drafts unpersisted."
  exit:
    natural: "The operator can continue from the created automation or choose a Task or automation creation path."
  abandonment:
    - at_step: 1
      how: "The operator applies a filter that matches nothing."
      resume: "Clear filters returns to the truthful zero-inventory introduction."
  crosses: [tasks, automation-jobs, automation-triggers, workspace-scope, empty-state-ui]
```
