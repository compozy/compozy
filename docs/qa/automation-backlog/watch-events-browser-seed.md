# Watch-events real-daemon browser seed harness

- Legacy ID: AB-009
- Source: J-16 / LP-040, LP-043, LP-044 / task-11 E2E-runtime + task-12 E2E-web
- Why automate: the runtime and component lanes exist, but `web/e2e/fixtures/*` has no seed that parks a real watch-events run and commits a matching durable event so Playwright can drive editor → park read-model → wake.
- Suggested layer: E2E browser (`make test-e2e-web`) + a daemon-side seed that arms a `task.status_changed`/`task.run.completed` subscription and commits a matching event.
- Spec sketch: seed an `events:` node parked in `watching`; drive catalog → editor → run → run detail; assert subscriptions, cursors, and `last_wake_at`; commit a matching event and assert wake plus downstream render; commit a non-matching or cross-workspace event and assert no wake. True end state: the browser matches the park/wake state without an optimistic-UI lie.
- Status: proposed
