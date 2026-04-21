- Goal (incl. success criteria):
- Restore daemon-backed ACP cockpit visibility for `thinking` and tool-call lifecycle states so transient updates are shown before terminal completion.
- Success means daemon-backed runs visibly preserve ACP progress states in order, remote attach/reconnect keeps the existing transcript baseline, focused regressions pass, and `make verify` passes.

- Constraints/Assumptions:
- Follow `AGENTS.md` and `CLAUDE.md`.
- Required skills active: `no-workarounds`, `systematic-debugging`, `bubbletea`, `golang-pro`, `testing-anti-patterns`; `cy-final-verify` before completion.
- Do not touch unrelated worktree changes.
- Root-cause fix only: no timing hacks, no protocol shims, no destructive git commands.

- Key decisions:
- Treat the main regression as UI-side loss of transient ACP state caused by snapshot coalescing, not ACP parsing loss.
- Keep daemon transport/public event contracts unchanged unless verification proves an additional issue.
- Hydrate remote incremental translation from snapshot state instead of rebuilding from an empty view after attach/reconnect.

- State:
- Completed and verified.

- Done:
- Re-read required skill instructions.
- Reconfirmed root cause from current code and pre-daemon code:
- ACP `session.update` still carries `agent_thought_chunk`, `tool_call`, `tool_call_update`, and pending/in-progress/completed states.
- `uiController.prepareDispatchBatch()` currently coalesces all `jobUpdateMsg` values per job index, so only the last snapshot survives.
- Pre-daemon UI consumed each translated `uiMsg` individually, so transient ACP states remained visible.
- Identified secondary continuity gap: remote bootstrap summary snapshots do not currently seed the translator state used for later live events.
- Persisted accepted plan to `.codex/plans/2026-04-20-acp-streaming-parity.md`.
- Added regressions in `internal/core/run/ui/adapter_test.go` for:
  - `tool_call` pending -> in_progress -> completed within one burst;
  - `thinking` followed by tool activity within one burst;
  - bootstrap snapshot hydration before live session updates.
- Added `LoadSnapshot` support in `internal/core/run/transcript/model.go` and coverage in `internal/core/run/transcript/model_test.go`.
- Reworked UI batching in `internal/core/run/ui/model.go` so `jobUpdateMsg` coalescing is semantic and lossless for visible ACP state transitions.
- Marked remote bootstrap `jobUpdateMsg` values as translator hydration baselines in `internal/core/run/ui/remote.go`.
- Removed race-prone global viewport test hooks by moving viewport content hooks onto the `uiModel` instance and updating affected tests.
- Verification passed:
  - `go test ./internal/core/run/ui ./internal/core/run/transcript -count=1`
  - `go test -race ./internal/core/run/ui -count=1`
  - `make verify`

- Now:
- Task complete; ready for handoff.

- Next:
- None.

- Open questions (UNCONFIRMED if needed):
- UNCONFIRMED: whether journal flush batching still causes noticeable latency after the UI fix. No evidence from current regressions or verification required a journal change.

- Working set (files/ids/commands):
- `.codex/plans/2026-04-20-acp-streaming-parity.md`
- `.codex/ledger/2026-04-20-MEMORY-acp-streaming.md`
- `internal/core/run/ui/{model.go,types.go,remote.go,timeline.go,update.go,adapter_test.go,view_test.go,update_test.go}`
- `internal/core/run/transcript/{model.go,model_test.go}`
- Commands:
- `git status --short`
- `go test ./internal/core/run/ui ./internal/core/run/transcript -count=1`
- `go test -race ./internal/core/run/ui -count=1`
- `make verify`
