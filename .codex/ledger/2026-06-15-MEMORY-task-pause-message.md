Goal (incl. success criteria):

- Implement pause + same-session message composer for running Compozy task jobs.
- Success means: an ACP-backed job can be paused via explicit job control, the TUI enables a message composer only while the selected job is paused, sending a message resumes via the same ACP session ID, events/snapshots/remote attach remain consistent, focused tests pass, `rtk make verify` passes, and `$cy-impl-peer-review` reaches SHIP.

Constraints/Assumptions:

- Follow AGENTS/CLAUDE: all shell commands via `rtk`, no destructive git commands, no unrelated reverts.
- Required skills loaded/read: brainstorming, tui-design, tui-glamorous, golang-pro, systematic-debugging, no-workarounds, testing-anti-patterns. Need `cy-final-verify` before completion.
- Worktree is already dirty with unrelated sidebar/task-number/token-layout work; preserve and layer changes on top.
- V1 pause = ACP `session/cancel` for the active turn, preserving ACP `sessionId`; message = new `session/prompt` in the same session.

Key decisions:

- Use job-scoped controls rather than run-global controls.
- Keep ACP details behind `internal/core/agent`; expose active attempt controls through a runtime `JobControlRegistry` carried by `RuntimeConfig`.
- Do not model pause as context cancellation, shutdown, retry, or terminal job cancellation.
- Persisted run row can remain `running`; job snapshot/event state carries `pausing` / `paused`.

State:

- Implementation in progress; mapping complete; foundation patches starting.

Done:

- Explored Compozy current TUI/executor/daemon/ACP flow.
- Explored `agh` busy-input model and `.resources/{claude-code,harnss,pi}`.
- Confirmed ACP supports `CancelNotification`, same-session `PromptRequest`, and UUID `messageId`.
- Persisted accepted plan to `.codex/plans/20260615-000000*task-pause-message.md`.
- Created active goal with peer-review success criterion.
- Re-read own ledger and scanned nearby TUI/task ledgers after compaction.
- Mapped runtime config, job lifecycle, ACP client/session, daemon API, snapshot builder, transcript, and TUI model.

Now:

- Patch typed control registry, event contracts, transcript/session update conversion.

Next:

- Wire ACP controller loop, daemon/API/client, TUI composer, tests, verify, peer review.

Open questions (UNCONFIRMED if needed):

- None.

Working set (files/ids/commands):

- `.codex/plans/20260615-000000*task-pause-message.md`
- `.codex/ledger/2026-06-15-MEMORY-task-pause-message.md`
- Expected source areas: `internal/core/model`, `internal/core/agent`, `internal/core/run/internal/acpshared`, `internal/core/run/executor`, `internal/core/run/ui`, `internal/daemon`, `internal/api/{contract,core,client}`, `pkg/compozy/events/kinds`.
