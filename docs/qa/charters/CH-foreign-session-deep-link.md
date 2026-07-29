# CH-foreign-session-deep-link: Press back at every point where a shared link could steal your workspace

```yaml
charter:
  id: CH-foreign-session-deep-link
  mission: "As Nia, open a teammate's session link from the wrong workspace and press back, forward, and refresh at every step — the confirmation must always be replayable from the URL, and nothing of the foreign session may appear before the answer."
  mode: charter-with-tour
  persona:
    name: Nia
    device: laptop
    network: wifi-fast
    locale: en-US
  journey: J-open-foreign-session
  scenarios: [ET-web-session-cross-workspace-confirm, ET-web-session-deep-link-isolation]
  tour: Back-Button Tour
  time_box_minutes: 60
  guidance:
    must_try:
      - "Bootstrap a fresh isolated lab, export COMPOZY_WEB_API_PROXY_TARGET from the bootstrap manifest (never a hardcoded port), register PIDs, and run eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every pass/fail/blocked/abort exit; cite clean teardown.json."
      - "With workspace A active and a visible window arranged, open a session owned by workspace B on the canonical deep link and again on the short permalink; both must show a confirmation naming B by its registered name and describing the switch as changing the active workspace and its windows."
      - "While the confirmation is pending, inspect the page for any foreign title, transcript, metadata, or window — only the owning workspace's identity may have been resolved. Confirm the network activity behind it is the minimal owner projection and that nothing merged into session catalogs or detail caches."
      - "Press back from the confirmation, then forward, then refresh mid-transition, then open the confirmation URL in a second tab: the confirmation must replay from the route rather than resolving from stale client memory, and no workspace may change without an answer."
      - "Confirm the switch and verify the active workspace becomes B, B's desktop arrangement is the one restored, and the session opens on the same route; then press back and confirm the resulting state is honest about which workspace is now active."
      - "Cancel on both routes and verify workspace A, its arrangement, and the not-found state are untouched, and that the declined state does not re-open the dialog on its own."
      - "Open a session id that exists in no workspace and confirm the not-found surface is unchanged with no confirmation offered; then open a session owned by A and confirm both routes open it directly with no confirmation."
      - "Try to inject an owning workspace through the URL and confirm the identity comes only from the owner lookup, never from a search parameter."
    must_avoid:
      - "Any agent-path or native-tool crossing — agents never cross through web routing; that is CH-cross-workspace-mode-seams."
      - "Re-running the open-fast latency matrix of J-12 or the blank-on-return matrix of J-11; only note a stall as a paper cut if it appears on this flow."
      - "Editing route search state by hand instead of reaching it through the confirmation UI, except for the deliberate injection attempt above."
  coverage:
    tier: targeted
    surfaces: [web-canonical-deep-link, web-short-permalink, workspaceSwitch-route-search, GET-/api/sessions/:session_id/owner, active-workspace-store, os-desktop-arrangement]
    invariants: [1]
    hot_spots:
      - "ADR-004's pre-confirmation exposure risk: the loader may fetch only the three-field owner projection, and no foreign session payload may reach a workspace-scoped cache before the answer."
      - "Invariant 1 for the web axis: the rewritten contract is 'never switches without confirmation', not 'never switches' — a stale not-found on a foreign link is now a defect, and so is a silent auto-switch."
      - "Route-ownership regression from ADR-008: the session leaf exclusively owns workspaceSwitch, so a parent route stripping it resurfaces as a confirmation that will not replay."
    adrs: [ADR-004, ADR-008]
    expected_evidence: "Captures of the pending confirmation with the page behind it, the post-confirm restored arrangement, and the post-cancel unchanged workspace; the URL bar at each back/forward/refresh step; and the network trace showing only the owner projection pre-confirmation."
    exit_criteria: "Both link forms confirm, switch, cancel, and replay correctly; no foreign session data appeared before an answer; no navigation gesture changed the active workspace on its own; and unknown ids still read as not found."
```

<!-- The charter is durable and immutable: re-run it in later cycles; each run's debrief belongs in that run's dated report. -->
