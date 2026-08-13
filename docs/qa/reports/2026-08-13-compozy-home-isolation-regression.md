# COMPOZY_HOME isolation regression QA report — 2026-08-13

## Scope

- Charter: `CH-compozy-platform-hard-cut`
- Scenario: `RT-compozy-home-isolation`
- Bug: `BUG-20260812-global-workspace-gateway-config`
- Persona: Bruno
- Tour: Garbage Tour
- Surfaces: CLI and daemon runtime

## Session matrix

| Session | Runtime home | Check | Verdict |
| --- | --- | --- | --- |
| 1 | Fresh isolated `COMPOZY_HOME` | Start with an operator-home global Gateway section, inspect status and workspaces, stop, then restart | Pass |
| 2 | Second isolated `COMPOZY_HOME` | Prove independent runtime state and repeat startup | Pass |
| 3 | Second isolated `COMPOZY_HOME` | Confirm a real project workspace still rejects a workspace-local Gateway section | Pass |

## Evidence

- Bootstrap manifest: `/Users/pedronauck/dev/qa-labs/compozy-compozy-home-isolation-regression-20260813-033540-246841-lab/qa-artifacts/qa/bootstrap-manifest.json`
- First start and restart: `qa-artifacts/qa/cli/first-status.json` and `qa-artifacts/qa/cli/restart-status.json`
- Second isolated runtime: `qa-artifacts/qa/cli/second-status.json`
- Real project overlay rejection: `qa-artifacts/qa/cli/project-overlay-rejection.json`
- Strict audit: `qa-artifacts/qa/qa-audit-report.json`
- Teardown: `qa-artifacts/qa/teardown.json` with `"clean": true` and no survivors

The first and second structured status documents selected different runtime homes and sockets while
reporting the same operator home as the default workspace. In both cases Gateway remained disabled in
the isolated runtime, proving the operator-home global section was not merged. Registering a separate
project root with its own Gateway section returned `gateway settings are global-only`, preserving the
workspace ceiling.

## Compozy Impact Audit

- Native tools: no descriptor or schema change; the walk checks existing workspace registration through the CLI.
- Extensibility and hooks: no extension, hook, capability, registry, bridge, or sidecar contract change; only synthetic default-workspace config classification is under test.
- Workspace data isolation: the selected runtime home remains global-scoped, while the operator-home default workspace is registered without importing its global config into the isolated runtime.
- Official Compozy skill: no public command, tool ID, or runtime-operation contract changed.
