# CH-desktop-release-rehearsal: E2E-019 fail-closed desktop release rehearsal (deferred from Task 05)

```yaml
charter:
  id: CH-desktop-release-rehearsal
  mission: "As Dora, rehearse the desktop release lane end to end — dry-run without signing secrets must hard-fail before publication; a draft release with full secrets publishes immutable payloads before one channel-beta ref-CAS; a simulated platform failure leaves the channel unchanged and explicit evidence."
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
  e2e: [E2E-032, E2E-033]
  lab:
    bootstrap: "eng-qa-bootstrap manifest for any local verification steps; the rehearsal itself runs in the release workflow's dry-run/draft path — never against the production channel"
    isolation: "local verification uses a unique COMPOZY_HOME from the manifest; default home/ports forbidden"
    web_proxy: "not applicable — no web surface in this session"
    pids: "register any local verifier processes at <QA_OUTPUT_PATH>/qa/pids/<name>.pid"
    config_writes: "config mutations run sequentially per QA home — never parallel"
    teardown: "eval \"$TEARDOWN_COMMAND\" (or make qa-reap) on every terminal path; cite teardown.json \"clean\": true (L-029); delete any draft release created by the rehearsal"
  guidance:
    must_try:
      - "Dry-run with required signing material absent → the desktop lane asserts material before building and hard-fails; no immutable asset or channel commit is published."
      - "Draft-release path with full secrets → immutable GitHub assets verify before one channel commit and ref CAS; the independent verifier checks each checksum and signature against the trusted public identity, then reads both platform manifests and one artifact per platform."
      - "Simulate one platform build failure → the channel ref stays unchanged, the GitHub release stays draft, and the operator gets explicit evidence — never a silently dropped platform; confirm the reserved `desktop/stable/` prefix stays blocked."
      - "E2E-033: repair both platform manifests to one known-good generation, verify every referenced immutable asset, record the audit commit, and prove an identical operation id is idempotent."
      - "Walk `REL-electron-cutover-announcement`: public notes and install guidance name Electron, beta-only availability, exact asset names, and the explicit portable-Linux old-AppImage removal step."
      - "Close the credential-custody gate against `docs/runbooks/desktop-release-credentials.md`: record backup inventory, access review, and one non-production rotation rehearsal."
    must_avoid:
      - "Never publish a public (non-draft) release, move the production channel ref, or exercise npm/Homebrew from this rehearsal — they belong to the authorized release run."
      - "This charter settles no tracker scenario; its verdict is release evidence recorded in the run report."
```

<!-- Immutable charter: each run's debrief belongs in that run's dated report. -->
