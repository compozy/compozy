Goal (incl. success criteria):

- Implement accepted plan for structured ACP/assistant-ui run/task logs in `web/`.
- Success means canonical backend transcript contract, web assistant-style rendering, task/run integration, tests, and clean `make verify`.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE rules: no destructive git commands, no workarounds, skills checked, `make verify` before completion.
- Worktree is dirty with unrelated/user changes; preserve them and integrate carefully.
- Accepted plan persisted at `.codex/plans/2026-04-28-structured-acp-web-logs.md`.
- Scope is all current run/task log views; read-only transcript visualization, no composer/approval flow.

Key decisions:

- Use backend canonical transcript contract rather than frontend-only reconstruction.
- Add assistant-ui/AI SDK dependencies by package-manager command.
- Keep old flattened snapshot transcript for compatibility.

State:

- Reopened for bug: past run detail in `web/` gets stuck in loading when trying to view logs.
- Root cause evidence: the affected run's DB has 3,917 events, including 3,795 `session.update` rows and ~66 MB of event payload JSON. `/events?limit=1` and `/api/runs/:id` are fast, but `/snapshot` and `/transcript` take ~7.6s because `Snapshot` replays all events and `Transcript` calls `Snapshot`.
- Compact projection fix implemented and verified.

Done:

- Read visible plan context and relevant prior ledgers.
- Persisted accepted Plan Mode plan.
- Checked required skills: `golang-pro`, `testing-anti-patterns`, `systematic-debugging`, `no-workarounds`, `design-taste-frontend`.
- Added `/api/runs/:run_id/transcript` contract, handler, client method, OpenAPI schema, generated TS types, and daemon projection from session snapshots.
- Added backend tests for structured transcript projection and client decode; focused Go tests pass for touched backend packages.
- Added assistant-ui read-only transcript panel, run detail integration, task detail compact related-run panel, mocks, stories, and route tests.
- `bun run --cwd web typecheck` passed.
- `bun run --cwd web test` passed: 42 files, 220 tests.
- `make verify` passed end to end after lazy-splitting the transcript panel and fixing jsdom/Playwright environment setup.
- Confirmed compact read evidence against affected live DB: all event payloads were ~66 MB, compact session reconstruction input is ~3.9 MB, and lifecycle snapshot input is ~1.6 KB.
- Added compact run DB reads, terminal snapshot compact projection, transcript reconstruction independent from dense snapshot, and regression coverage.
- Focused commands passed after compact projection changes:
  - `go test ./internal/store/rundb -run 'TestRunDBCompactHistoricalReadsAvoidUnboundedSessionPayloads|TestRunDBListEventsRespectsLimitAndHasMore' -count=1`
  - `go test ./internal/daemon -run 'TestRunManagerHistoricalSnapshotAndTranscriptUseCompactProjection|TestRunManagerSnapshotIncludesJobsTranscriptAndNextCursor' -count=1`
  - `go test ./internal/store/rundb ./internal/daemon -count=1`
  - `go test ./internal/api/client ./internal/api/core ./internal/api/contract ./internal/api/httpapi ./pkg/compozy/runs -count=1`
  - `go vet ./internal/store/rundb ./internal/daemon`
- `bun run --cwd web test -- src/storybook/route-stories.test.tsx` passed after preventing route stories from opening real workspace WebSockets in tests.
- `make verify` passed end to end after compact projection and test WebSocket override changes. Evidence: frontend lint/typecheck/tests/build passed, Go lint passed with 0 issues, 2,757 Go tests passed with 2 skipped, Go build succeeded, and Playwright e2e passed 5/5.

Now:

- Final response.

Next:

- None.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/2026-04-28-structured-acp-web-logs.md`
- `.codex/ledger/2026-04-28-MEMORY-structured-web-logs.md`
- Target areas: `internal/api/contract`, `internal/api/core`, `internal/daemon`, `internal/store/rundb`, `web/src/systems/runs`, `web/src/systems/workflows`, `packages/ui`
- Focused command passed: `go test ./internal/daemon ./internal/api/client ./internal/api/core ./internal/api/contract ./internal/api/httpapi ./pkg/compozy/runs -count=1`
- Full command passed: `make verify`
- Live evidence command: `curl -o /dev/null /api/runs/<run>/snapshot` on port 2323 returned 200 but took ~7.6s; transcript took ~7.7s and ~10 MB.
