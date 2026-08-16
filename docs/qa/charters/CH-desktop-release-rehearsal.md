# CH-desktop-release-rehearsal: E2E-019 fail-closed desktop release rehearsal (deferred from Task 05)

```yaml
charter:
  id: CH-desktop-release-rehearsal
  mission: "As Dora, rehearse the desktop release lane end to end — dry-run without signing secrets must hard-fail before any publish; a draft release with full secrets publishes payloads then manifests and re-verifies them publicly; a simulated platform failure publishes no feed and leaves explicit evidence — implemented in Task 05, executed here per the loop directive."
  mode: strategy-based
  platform: any (GitHub Actions release workflow; no local webview involved)
  persona:
    name: Dora
    device: desktop
    network: wifi-fast
    locale: en-US
  journey: J-publish-compozy-beta
  scenarios: [REL-beta-channel-contract, REL-channel-repair-known-good, REL-electron-cutover-announcement]
  tour: Garbage Tour
  time_box_minutes: 90
  e2e: [E2E-019]
  lab:
    bootstrap: "eng-qa-bootstrap manifest for any local verification steps; the rehearsal itself runs in the release workflow's dry-run/draft path — never against production feeds"
    isolation: "local verification uses a unique COMPOZY_HOME from the manifest; default home/ports forbidden"
    web_proxy: "not applicable — no web surface in this session"
    pids: "register any local verifier processes at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029); delete any draft release created by the rehearsal"
  guidance:
    must_try:
      - "Dry-run with signing/feed-key material absent → the desktop lane asserts material before building and hard-fails; nothing reaches the update feed (US-021.AC-1)."
      - "Draft-release path with full secrets → immutable GitHub assets verify before one channel commit and ref CAS; the post-publish verifier reads both platform manifests and one artifact per platform; the sole finalizer publishes only after verification."
      - "Simulate one platform build failure → no feed published, the GitHub release stays draft, and the operator gets explicit evidence — never a silently dropped platform (US-021.EC-1); confirm the reserved `desktop/stable/` prefix stays blocked (US-021.EC-2)."
      - "Close the human gate: the feed-key custody runbook (backup + rotation) exists and has been reviewed — record the checklist evidence (US-021.AC-3)."
    must_avoid:
      - "Never publish a public (non-draft) release, never touch the production feed domain, never exercise npm/Homebrew from this rehearsal — they belong to the staging job, not the finalizer."
      - "This charter settles no tracker scenario; its verdict is release evidence recorded in the run report."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
