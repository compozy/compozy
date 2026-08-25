# CH-agent-session-control: Typed Goal control from an agent session

```yaml
charter:
  id: CH-agent-session-control
  mission: "As Ada, manage a child Goal with typed native, HTTP, UDS, and CLI operations and prove workspace and lineage authorization without changing immutable origin policy."
  mode: scenario-based
  persona:
    name: Ada
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-29
  scenarios: [GL-agent-session-control]
  tour: Error Guessing Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Set and replace a child Goal with an explicit provider/model/reasoning/speed runtime, then read status and clear it through every structured surface."
      - "Use the caller session as the target, an authorized descendant as the target, and an unrelated same-workspace session as a negative control."
      - "Compare HTTP, UDS, CLI JSON/JSONL, and compozy__goal_control outcome, reason_code, snapshot, and content type."
      - "Confirm failed binding evidence remains queryable and origin/network provenance does not change when the worker runtime changes."
    must_avoid:
      - "Using slash-command prompt text to impersonate another agent session or reading SQLite directly."
    evidence_expectations:
      - "Fresh structured payloads for each operation, authorization denial, and provenance read after refresh/reconnect."
```
