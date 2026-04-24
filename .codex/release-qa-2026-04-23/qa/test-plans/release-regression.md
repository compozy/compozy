# Release Regression Suite

## Suite Tiers

| Suite    | Duration  | Frequency               | Coverage                                                                                                         |
| -------- | --------- | ----------------------- | ---------------------------------------------------------------------------------------------------------------- |
| Smoke    | 15-30 min | Every release candidate | `compozy --help`, `daemon start/status/stop`, `workspaces resolve/list`, `tasks run --dry-run`, Web UI dashboard |
| Targeted | 30-60 min | Current change          | Codex `gpt-5.5` default resolution, config precedence, docs/help examples, daemon Web UI start/archive flow      |
| Full     | 2-4 hours | Release close           | `make verify`, Playwright E2E, realistic temp Node API project, docs/config scan                                 |
| Sanity   | 10-15 min | After final fix         | Focused Go/TS tests for touched files plus selected CLI/browser smoke                                            |

## Execution Order

1. Smoke: environment/tool versions, build binary, CLI help, daemon start/status/stop.
2. P0: `make verify`, Codex default model resolution, daemon-served Web UI Playwright smoke.
3. P1: temp Node API project workflow, browser-use navigation, docs/config consistency scan.
4. P2: release config audit and additional responsive/browser evidence.
5. Exploratory: inspect logs, unexpected warnings, and mismatched documentation examples.

## Pass/Fail Criteria

- PASS: all P0 pass, no Critical/High bugs open, final `make verify` exits 0, and runtime/browser evidence is fresh.
- FAIL: any P0 fails, `make verify` fails, live daemon cannot start, or Codex default remains `gpt-5.4` in active surfaces.
- CONDITIONAL: P1 blocked only by unavailable live credentials/adapters, with local deterministic proof passing and blocker documented.

## Automation Follow-Up

- Any manual public regression discovered during this run must become Go, TS, or Playwright coverage when the existing harness can express it.
- Live LLM credential/adaptor gaps stay blocked, not mocked as final proof.
