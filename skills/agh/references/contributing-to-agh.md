# Contributing To AGH

## Contents

- Repository posture
- Go runtime rules
- Public surfaces
- Cross-surface impact audit
- Error and context discipline
- Tests
- Dirty worktrees
- Documentation impact
- Codegen and boundaries
- Workflow lessons

## Repository Posture

AGH is greenfield alpha. There are no production users, and backward compatibility must not reduce code quality. Prefer hard cuts over compatibility bridges:

- no aliases for renamed public concepts
- no dual fields
- no schema fallbacks for old state
- no defensive compatibility paths unless a written plan explicitly requires them

Never run destructive git commands without explicit user permission.

## Go Runtime Rules

Before editing Go runtime code, read the local repository instructions and the relevant internal/CLAUDE.md section. Core invariants:

- internal/daemon is the composition root.
- Packages must not import daemon, api, or cli.
- Interfaces are defined where consumed.
- Direct calls through interfaces beat event-bus style routing.
- Long-running work must detach from request contexts intentionally.
- Public runtime features must be agent-manageable.

Use structured errors with wrapping. Do not discard errors with \_ in production or tests.

## Public Surfaces

Any change touching a public surface should close the loop:

- contract and generated references when applicable
- HTTP/UDS handlers when state crosses daemon boundaries
- CLI command/client support
- tool or skill surfaces for agent operation
- docs
- tests

Backend-only work may declare no web/docs impact only after analysis.

## Cross-Surface Impact Audit

Every feature, bug fix, refactor, public contract change, CLI/API/native-tool/config/docs update, or runtime behavior change needs an `AGH Impact Audit` before it is claimed complete:

- Native tools: affected `agh__*` IDs, toolsets, descriptors, schemas, risk flags, availability diagnostics, capability gates, tests, or explicit no-impact evidence.
- Extensibility and hooks: affected extensions, hook taxonomy/dispatch, skills/capabilities, tools/resources, bundles, registries, bridge SDKs, MCP sidecars, config lifecycle, docs, tests, or explicit no-impact evidence.
- Workspace data isolation: global/workspace/session/agent scope decision, `workspace_id` propagation through CLI/HTTP/UDS/core/store/web/SSE/cache/events, cross-workspace leak tests when data is listed/read/cached/emitted, or explicit no-impact evidence.
- Official AGH skill: updates to `skills/agh/SKILL.md` or `skills/agh/references/*.md` when public behavior, tool IDs, CLI paths, hook events, capabilities, bundles/resources, memory/network/task semantics, or agent guidance changes, or explicit no-impact evidence.

Worktree or QA lab isolation is not a substitute for workspace data isolation. `No impact` must name the checked surfaces and why they remain unchanged.

## Error And Context Discipline

Use context.Context consistently for runtime operations. Do not tie detached prompts, network sends, automation jobs, or session work to a request lifetime unless cancellation is the intended behavior.

Never log or return raw claim tokens, provider secrets, OAuth codes, PKCE verifiers, MCP credentials, sandbox internals, or secret-shaped environment values.

## Tests

Every task requires a test decision. Before adding, moving, or broadening tests, name:

- the invariant
- the owning layer
- the canonical suite

Default to updating an existing canonical suite. Do not add tests that only freeze implementation details, snapshots, generated output, config shape, CSS literals, or file existence unless that artifact itself is the product contract.

Common static-artifact rejections include Storybook setup/config/bootstrap, CI workflow/action YAML, Mage/Make/package-script plumbing, generic config/file existence, generated-output drift, snapshots/goldens, prose literals, and coverage-padding. Allow these only when the artifact itself is the product/operator/agent contract and no stronger gate owns the invariant.

When a test reveals broken production behavior, fix production code. Do not weaken the test to match the bug.

## Dirty Worktrees

Assume unrelated changes belong to the user or another agent. Do not revert them. If unrelated, ignore them. If they affect the task, read and work with them.

## Documentation Impact

Public wording follows COPY.md; visual and UI guidance follows generated DESIGN.md and token source files. Runtime docs must describe behavior the daemon actually supports, not aspirational behavior.

## Codegen And Boundaries

`make verify` is the broad gate and includes codegen checks, frontend checks, Go fmt/lint/test/build, and `make boundaries`. For focused codegen work, `make codegen` regenerates OpenAPI, SDK contracts, generated design regions, the config lifecycle matrix, and the native tool catalog; `make codegen-check` verifies they are current.

The codegen CLI subcommands are `openapi`, `sdk-contracts`, `lifecycle-matrix`, `native-tool-catalog`, `all`, and `check`. Do not hand-edit generated lifecycle matrix or native tool catalog output to make checks pass.

Run `make boundaries` after package graph or composition-root changes. Boundary failures are architecture defects, not lint preferences.

## Workflow Lessons

Read adjacent institutional memory before changing a surface it covers:

- `docs/_memory/standing_directives.md#sd-011--extensible-and-agent-manageable-by-design` for extensibility and agent-operable surfaces.
- `docs/_memory/lessons/L-009-concurrent-worktree-deadlock.md` for parallel worktree and QA isolation.
- `docs/_memory/lessons/L-016-native-provider-qa-home-policy.md` for provider-home policy during native-provider QA.
- `docs/_memory/lessons/L-022-eyebrow-canonical-source.md`, `L-023-token-utility-canonical-form.md`, and `L-024-design-md-generated-tokens.md` for design-system and generated-token work.
