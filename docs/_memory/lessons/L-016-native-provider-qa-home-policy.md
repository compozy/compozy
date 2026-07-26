# L-016: Native Provider QA Must Respect Home Policy

**Class:** Testing / Workflow

## Incident

Local QA guidance generalized provider-home isolation across every provider lane.
That made release/scenario QA instructions tell operators to launch provider-backed
commands with `HOME="$PROVIDER_HOME"` and `CODEX_HOME="$PROVIDER_CODEX_HOME"`
from the bootstrap manifest, even for direct native providers such as Claude Code.

The product contract did not actually require that. Direct native providers already
declare whether they should use the operator home or an AGH-owned provider home
through `home_policy`. For `native_cli` providers with `home_policy=operator`,
rewriting `HOME` in the QA harness changed the runtime contract under test: AGH no
longer used the operator's installed/login-capable CLI state and instead exercised
an artificial isolated home that the real product path never needed.

This produced a false blocker during Claude Code QA:

- isolated QA lane: `session new` succeeded, but the first prompt failed with
  `Authentication required`
- operator-home lane: preserving the user's real `HOME` let the same AGH build
  create a real Claude ACP session, run tools, and write files successfully

## Root cause

QA isolation policy and provider auth policy were conflated.

- `AGH_HOME` / ports / sockets need isolation for deterministic QA.
- Provider auth state must still follow the provider contract.
- The documentation and QA skills treated `PROVIDER_HOME` as mandatory for all
  provider-backed commands instead of only for bound-secret, brokered, or
  explicitly isolated-home lanes.

The runtime itself already modeled the distinction correctly:

- `ProviderHomePolicyOperator` is the default for providers with no explicit
  override.
- `ApplyHomePolicy` rewrites `HOME` only when `home_policy=isolated`.

The misleading part was the QA harness guidance and any harness code that
hardcodes `HOME` independently of the provider contract.

## Fix / Rule

When testing native providers, isolate **AGH runtime state** and **provider auth
state** separately:

- Always isolate `AGH_HOME`, daemon ports, sockets, and other AGH-owned runtime
  state for QA.
- Use `PROVIDER_HOME` / `PROVIDER_CODEX_HOME` only for:
  - `bound_secret` providers
  - brokered/shared credentials
  - scenarios that explicitly validate `home_policy=isolated`
- For `native_cli` providers with `home_policy=operator`, preserve the operator's
  real `HOME` / native login state. Testing them against an invented isolated
  home is a different scenario and must be called out explicitly.

Never let a hermetic QA harness silently change the provider contract under test.
If the harness overrides `HOME`, it must do so because the scenario explicitly
asked for isolated native auth, not because isolation is the blanket default.

## Evidence

- Root instructions updated:
  - `AGENTS.md`
  - `CLAUDE.md`
- QA skill guidance updated:
  - `.agents/skills/agh/agh-qa-bootstrap/SKILL.md`
  - `.agents/skills/agh/agh-qa-bootstrap/references/bootstrap-contract.md`
  - `.agents/skills/qa-execution/SKILL.md`
  - `.agents/skills/agh/real-scenario-qa/SKILL.md`
- Final QA plan corrected:
  - `.compozy/tasks/final-qa/_master-qa-plan.md`
  - `.compozy/tasks/final-qa/_children/03-acp-sessions.md`
  - related child plans for API/UI/docs/autonomy/memory/network/observability
- Provider home policy in runtime:
  - `internal/config/provider.go`
  - `internal/providerenv/env.go`
  - `internal/session/provider_runtime.go`
- Harness code to audit when adding more automated native-provider QA:
  - `internal/testutil/e2e/runtime_harness.go`
- Session evidence:
  - `.codex/ledger/2026-05-03-MEMORY-claude-finalqa-smoke.md`
  - `/Users/pedronauck/dev/qa-labs/agh-claude-finalqa-smoke-20260504-023728-898221-lab/qa-artifacts/qa/verification-report.md`
