# CH-session-message-delete-integrity: Do session messages and owned history survive reload and leave cleanly on delete?

```yaml
charter:
  id: CH-session-message-delete-integrity
  mission: "As Théo, send one durable user message, return through a fresh read, then delete the session and prove the transcript is neither lost before deletion nor stranded afterward."
  mode: scenario-based
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-11
  scenarios: [RT-session-message-reload, RT-session-delete-owned-history]
  tour: Interrupt Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Send a uniquely named user message through a public session surface, leave the thread, and return through a fresh detail/transcript read; the exact message must remain in chronological order."
      - "Delete the same stopped session through a public surface and confirm list, detail, transcript, permission, token-usage, and event reads no longer expose session-owned history."
      - "Keep a second session in the same workspace and prove its transcript and usage remain intact after the first session is deleted."
    must_avoid:
      - "Reading SQLite directly as persona evidence or accepting an optimistic Web row without a fresh public read."
  evidence_expectations:
    - "Structured CLI or HTTP outputs before and after reload/delete, plus a Web screenshot when the surface is available."
    - "A neighboring-session control proving deletion is scoped to the exact session identity."
  truthful_ui_check: "Reload must not erase or reorder the user message, and delete must not leave a ghost session or claim success while owned history remains publicly readable."
```
