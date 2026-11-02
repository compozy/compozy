# Session monitor kind: isolated lab

Branch `feat/session-monitor-kind`. Isolated daemon with `acpmock` agents: owners register monitors through the
real `compozy__monitor_create` tool, and targets hold a turn until canceled or stopped. Replay script and
output: `artifacts/2026-11-02-session-monitor-kind/`. Teardown reported `TEARDOWN_ALL_CLEAN=true`.

| Case                                   | Result                                                                               |
| -------------------------------------- | ------------------------------------------------------------------------------------ |
| `until=all`, first target settles      | Monitor stays `active`, owner badge `waiting`                                        |
| `until=all`, second target settles     | `fired`, `condition_met`, both targets `done`; one wake turn                         |
| `until=any`, operator stops one target | `fired`; stopped target matched, the other `running` and unmatched                   |
| `timeout_seconds=3`                    | `fired`, `timeout`, target `running` and unmatched                                   |
| Daemon restart with an active monitor  | `monitor.interrupted` then `monitor.recovered`; fired only after boot; one wake turn |

Earlier lab runs failed for lab reasons only: an unregistered workspace, a binary overwritten while running, no
`timeout` on macOS, and a fixture without the `sonnet` model alias.

Not covered: a real provider target that auto-continues after a restart. The mock target's interrupted turn ended,
so it read `done` at recovery. The Web divider and popover are covered by their suites, not by a browser walk.
