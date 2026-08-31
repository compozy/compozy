# CH-web-task-result-copy: Read and copy one large result without losing context

```yaml
charter:
  id: CH-web-task-result-copy
  mission: "As Cora, open a completed task, inspect a large result a page at a time, and copy the full value without a frozen screen or broken text."
  mode: charter-with-tour
  persona:
    name: Cora
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-complete-partial-loop
  scenarios: [TA-web-task-result-disclosure]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Enter from task Overview and from the task-run deep link; refresh both before opening the result."
      - "Open, page forward and back, copy the complete value, and confirm multibyte text remains intact."
      - "Observe loading and one recoverable failed read without using browser developer tools as the verdict."
    must_avoid:
      - "Opening an internal API URL that Cora cannot discover from the product."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
