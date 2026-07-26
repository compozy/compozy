# Bridge Web setup and remediation harness

- Legacy ID: AB-013
- Source: J-complete-web-bridge-setup / NB-026, NB-028, NB-039, NB-web-bridge-setup / CH-web-bridge-setup
- Why automate: the browser setup path combines daemon-derived checklist state, secret lifecycle, manifest copy, inline verification remediation, Telegram webhook registration, and two deliberately different delivery checks. Component tests cannot prove that a real daemon remains the source of truth after reloads, failed verification, and back-button navigation.
- Suggested layer: real-daemon Playwright E2E with isolated fake-provider endpoints and a seeded verification failure.
- Spec sketch: create a disabled Slack bridge, bind secrets, copy the generated manifest, fail and repair one verification check, reload and navigate back, register a Telegram webhook, run `Check target (dry run)`, then send one real test message. Assert the checklist only advances from daemon responses and the two delivery actions remain visibly distinct.
- Status: proposed
