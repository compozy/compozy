# CH-session-calm-transcript: Audit a dense transcript without card bulk

```yaml
charter:
  id: CH-session-calm-transcript
  mission: "As Théo, supervise a tool-heavy session whose settled work collapses into calm semantic summaries while failures, interruptions, plans, and changed files remain individually truthful."
  mode: charter-with-tour
  persona:
    name: Théo
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-14
  scenarios: [ET-web-session-transcript-calm-grammar]
  tour: Feature Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Create settled, live-tail, failed, and interrupted turns; expand summaries and compare command/file counts with the transcript payload."
      - "Exercise TodoWrite, runtime-event grouping, long user text, changed-file expansion, and the eight-file cap."
      - "Reload the finished session and prove folds, failures, receipts, and semantic summaries retain the same meaning."
    must_avoid:
      - "Counting duplicate edits as distinct files or hiding failed calls inside a settled summary."
```
