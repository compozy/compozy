# CH-coordination-future-runs: Adopt coordination from the run that needs it

```yaml
charter:
  id: CH-coordination-future-runs
  mission: "As Bruno, accept or dismiss the contextual coordination invitation and prove only future runs in the selected scope gain a bounded, truthful conversation while task state remains authoritative."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-enable-coordinated-conversations
  scenarios: [NB-coordination-invitation-future-runs, NB-run-conversation-bounds-usage]
  tour: Back-Button Tour
  time_box_minutes: 90
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab with unique AGH_HOME/ports/provider home/tmux socket, register PIDs, export AGH_WEB_API_PROXY_TARGET from the manifest, and execute eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite clean teardown.json."
      - "Drive the run-detail invitation, dismissal, acceptance, conversation, pagination, and usage panels through Playwright/browser-use at 375/768/1280 widths with keyboard-visible focus and non-color-only state. Use Back/Forward, refresh, a direct run URL, and double-click acceptance; API-only evidence cannot settle the UI."
      - "Exercise the invitation visibility matrix: active coordinator plus multiple workers, single-agent run, terminal run, Network disabled, dismissed scope, task vs workspace choice, and a second workspace with the same channel name."
      - "Compare the active run's immutable snapshot with a newly started coordinated run and an unrelated scope; then add conversation evidence and verify empty-silence copy, paginated history, SSE updates, actual-or-usage_unavailable labels, and task/claim/review authority on fresh reads."
    must_avoid:
      - "Patching an in-flight snapshot, treating conversation text as a task transition, reusing browser-local dismissal state, or planning mailbox/spend-cap behavior."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in the dated report. -->
