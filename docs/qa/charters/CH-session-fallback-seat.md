# CH-session-fallback-seat: A work session moves to the next seat only before acceptance and remembers it

```yaml
charter:
  id: CH-session-fallback-seat
  mission: "As Rafa, make a work session's first seat refuse before ACP acceptance and prove the prompt binds on the next seat with one ledger event and one marker per refused route, never advances after acceptance, and resumes on the accepted seat (or restarts with context replay when that seat is gone)."
  mode: charter-with-tour
  persona:
    name: Rafa
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-route-background-work
  scenarios: [RT-session-fallback-chain]
  tour: Network Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Author `seat-reviewer` with a refusing primary and a healthy `--fallback-route` carrying `command=`; `agent info -o json` shows `command_fingerprint`, never secrets beyond the command text the operator typed."
      - "First prompt binds on seat two: one `session.fallback.used` (phase bind, `sha256:` fingerprint, no command text), one `use_fallback` marker, no stop event; `runtime.effective` is seat two."
      - "A rate limit after acceptance keeps `retry` and writes no fallback event."
      - "Stop/resume lands on seat two; remove the route, stop/resume again: restart on the primary with `accepted_route_missing`."
    must_avoid:
      - "Background-role routing — CH-role-fallback-boundary owns roles."
```

<!-- The charter is durable and immutable: each run's debrief belongs in that run's dated report. -->
