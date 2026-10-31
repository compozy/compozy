# Issue #1068 lab evidence: stdio MCP process reuse

Date: 2026-10-31. Host: macOS (darwin arm64). Branch `fix/issue-1068-stdio-mcp-reuse` versus base
`37439a2c9` (main). Both binaries built from source with `go build ./cmd/compozy`.

## Fixture

A Go stdio MCP server built against the repository's `go-sdk` v1.8.0. On every launch it appends one
line to a launch log, writes `slow-lab: booting (sleep 3s)` to stderr, sleeps 3 seconds, then serves
one `echo` tool. `{"message":"crash"}` makes it write `slow-lab: crashing on request` to stderr and
exit 7.

Each run uses a fresh `COMPOZY_HOME`, its own HTTP port, a clean environment (`env -i`, no
`COMPOZY_*` variables), and this config:

```toml
[http]
port = 47681

[tools.policy]
external_default = "enabled"

[[mcp_servers]]
name = "slow"
command = "/tmp/lab1068/slow-mcp"
env = { LAB_LAUNCH_LOG = "/tmp/lab1068/<run>-launches.log", LAB_MODE = "slow" }
```

## Reuse run

Script: `compozy daemon start`; 5 × `compozy tool search echo -o json`; 10 ×
`compozy tool invoke mcp__slow__echo --input '{"message":"call-N"}'`;
`compozy tool invoke compozy__mcp_status --input '{"server_name":"slow"}'`; `compozy daemon stop`.

| Measure                               | Base `37439a2c9`                                   | Branch                                  |
| ------------------------------------- | -------------------------------------------------- | --------------------------------------- |
| Tool calls succeeded                  | 10 / 10                                            | 10 / 10                                 |
| Server launches                       | 43                                                 | 1                                       |
| Wall time for searches + calls        | 127 s                                              | 1 s                                     |
| `compozy__mcp_status` runtime         | `runtime_unavailable`, `call_timed_out` (2 s probe) | `ready`, probe `succeeded`, 1 tool      |
| `compozy-mcp-*` dirs added to `$TMPDIR` | 0                                                | 0                                       |
| Pool homes after `daemon stop`        | n/a                                                | 0                                       |
| `slow-mcp` processes after stop       | 0                                                  | 0                                       |

On the branch the single launch logged
`home=<COMPOZY_HOME>/cache/mcp/homes/compozy-mcp-15b2690aa4549f02-…` and
`npm=<COMPOZY_HOME>/cache/mcp/npm` (persistent package cache).

## Crash and backoff run (branch)

1. `tool invoke mcp__slow__echo '{"message":"warm"}'` → `echo: warm`; 1 launch.
2. `tool invoke mcp__slow__echo '{"message":"crash"}'` → `tool backend failed`.
3. Immediately, `compozy__mcp_status` → `State: runtime_unavailable`,
   `Diagnostic: mcp: call upstream server: mcp stdio server is backing off after a failure; next
   launch in 1s; stderr: slow-lab: booting (sleep 3s) … crashing on request`. Launch count stayed 1.
4. After the backoff window, `tool invoke mcp__slow__echo '{"message":"after-backoff"}'` →
   `echo: after-backoff`; launch count 2.
5. `daemon stop` → pool homes 0, `slow-mcp` processes 0.
