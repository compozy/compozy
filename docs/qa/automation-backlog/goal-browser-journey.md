# Goal real-daemon browser journey harness

- Legacy ID: AB-010
- Source: J-26, J-27 / GL-001..GL-016, GL-022..GL-024, GL-033, GL-035 / `_tests.md` E2E-web 1..10
- Why automate: the complete composer → session chip → Run timeline → editor flow is not pinned against one real daemon. Repeated release cycles should not depend on manual seed assembly.
- Suggested layer: E2E browser (`make test-e2e-web`) with daemon-side Goal session/editor seed fixtures.
- Spec sketch: start at `{goal:null}`, submit a Goal, drive two rejections and approval, pause/resume, replace/draft, external discovery, clear, turn pagination, Runs origin filter, and one editor-authored snapshot-pinned Run. True end state: reload shows daemon-owned truth with no polling, duplicate turn, fabricated context, stale replacement, or resurrected history.
- Status: proposed
