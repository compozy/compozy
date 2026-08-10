# CH-local-stream-auth-clean: Keep local streamed navigation free of remote auth errors

```yaml
charter:
  id: CH-local-stream-auth-clean
  mission: "As Bruno, navigate and reload streamed apps through the local Web and desktop shell, proving local work stays live without remote ticket requests or product console errors."
  mode: charter-with-tour
  persona:
    name: Bruno
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-operate-desktop-shell
  scenarios: [ET-web-desktop-shell-lifecycle, ET-web-window-routing-lifecycle]
  tour: Feature Tour
  time_box_minutes: 30
  guidance:
    must_try:
      - "Open the local desktop, Loops, and one live-stream view; switch through dock or shell entry points, then reload and revisit the same routes."
      - "Capture network and console evidence proving discovery uses `/api/status`, never `POST /api/gateway/stream-tickets`, and produces no Compozy-owned 4xx or 5xx response."
      - "Repeat the navigation through the native desktop app against the same isolated daemon and confirm route and window state remain truthful."
      - "Use an extension-free browser profile so injected content-script warnings cannot be mistaken for Compozy failures."
    must_avoid:
      - "Do not enable a remote gateway tier or suppress console entries; this charter owns the honest local-listener path."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief goes in that run's report (Session Debriefs), never here. -->
