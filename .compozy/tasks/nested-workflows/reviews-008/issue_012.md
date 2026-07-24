---
provider: manual
pr:
round: 8
round_created_at: 2026-07-24T19:59:29Z
status: resolved
file: internal/cli/daemon_commands.go
line: 1977
severity: medium
author: claude-code
provider_ref:
---

# Issue 012: Attach-mode boolean flags gate on presence, so =false applies

## Review Comment

`resolveTaskPresentationMode` gates the attach-mode flags on presence alone and
then applies the value unconditionally:

```go
if !commandFlagChanged(cmd, item.name) {
    continue
}
mode = item.value
explicitModes++
```

`ui`, `stream`, `detach` and `background` are real `cmd.Flags().Bool(...)`
booleans (daemon_commands.go:693-695, reviews_exec_daemon.go:165-167, 208-211), so
they accept `=false`. `attach` at line 1965 has the same shape.

Concrete failures:

- `compozy tasks run my-feature --detach=false` still resolves to detach mode and
  starts the run unattached — the exact `--flag=false` mode-switch bug that commit
  `6eed279` fixed for the parallel flags.
- `compozy tasks run my-feature --ui=false --stream` counts `explicitModes == 2`
  and errors "choose only one of --attach, --ui, --stream, or --detach" even
  though only one mode was requested.
- Same pattern at `internal/cli/reviews_exec_daemon.go:836`, where
  `--format json --ui=false` wrongly errors "cannot combine json output with ui
  attach mode".

Scripts that pass `--detach=$FLAG` are silently mis-routed.

Fix: read the value via `cmd.Flags().GetBool(item.name)` and only apply and count
the mode when it is true — the same value-gating already applied to
`--parallel` / `--parallel-tasks` / `--parallel-task-groups` / `--new`.
`--multiple` (string) and `--parallel-limit` (int) correctly stay presence-gated.

## Triage

- Decision: `VALID`
- Root cause: `resolveTaskPresentationMode` (daemon_commands.go:1958) gates the
  attach-mode boolean flags (`ui`, `stream`, `detach`, `background`) with
  `commandFlagChanged`, which reports whether the flag was *supplied*, not its
  value. Because these are real `cmd.Flags().Bool(...)` booleans, passing
  `--detach=false` (or `--ui=false`, etc.) sets `Changed()` to true, so the loop
  unconditionally applies `mode = item.value` and increments `explicitModes`.
  Result: `--detach=false` still routes to detach mode, and `--ui=false --stream`
  counts two explicit modes and errors "choose only one …". This is the exact
  `--flag=false` mode-switch class of bug that commit `6eed279` fixed for the
  `--parallel` / `--parallel-tasks` / `--parallel-task-groups` / `--new` flags by
  value-gating (`commandFlagChanged(...) && s.<bound>`).
- Secondary manifestation: `resolveReviewWatchPresentationMode`
  (reviews_exec_daemon.go:834) has the same presence-gate on `--ui`, so
  `--format json --ui=false` wrongly errors "cannot combine json output with ui
  attach mode".
- Fix approach: for the unbound bool flags, read the value with
  `cmd.Flags().GetBool(name)` after confirming the flag changed, and only apply /
  count / reject the mode when the value is `true`. `--attach` (string) stays
  presence-gated because its value is meaningful and already flows into `mode`;
  `--multiple` (string) and `--parallel-limit` (int) correctly stay
  presence-gated elsewhere.
- Cross-file note: `reviews_exec_daemon.go` is outside the single scoped code
  file, but the issue explicitly lists the `reviews_exec_daemon.go:836` failure
  as part of this defect. The change there is the minimal value-gate needed to
  fully resolve the reported behavior; no unrelated code was touched.
- Notes: Fixed both call sites and added regression tests covering
  `--detach=false`, `--ui=false --stream`, and the review-watch
  `--format json --ui=false` path.
