# Lessons Learned

Durable engineering lessons distilled from real AGH incidents and decisions since April 2026. Each lesson is a single file with a confirmed root cause, the fix or rule, and the evidence trail.

These are NOT speculative warnings — every lesson here has either an ADR, a commit, a review issue, or a verifiable QA bug behind it.

## Index

| ID                                                            | Title                                                                            | Class                            |
| ------------------------------------------------------------- | -------------------------------------------------------------------------------- | -------------------------------- |
| [L-001](L-001-detached-prompt-lifetime.md)                    | HTTP request lifetime ≠ prompt execution lifetime                                | Concurrency / API                |
| [L-002](L-002-tparallel-vs-tsetenv.md)                        | `t.Parallel()` is incompatible with `t.Setenv`                                   | Testing                          |
| [L-003](L-003-task-runs-single-queue.md)                      | `task_runs` is the single durable work queue                                     | Architecture / Autonomy          |
| [L-004](L-004-manual-equals-peer.md)                          | Manual operator paths converge with autonomous on same primitives                | Architecture / Autonomy          |
| [L-005](L-005-authoritative-primitive-exclusivity.md)         | Authoritative primitives are exclusive — observe ≠ own                           | Architecture                     |
| [L-006](L-006-greenfield-delete-not-adapt.md)                 | Greenfield + zero-legacy means delete, not adapt                                 | Project posture                  |
| [L-007](L-007-e2e-follows-runtime-contract.md)                | E2E harness regressions follow runtime contract changes                          | Testing                          |
| [L-008](L-008-schema-migrations-mandatory.md)                 | Schema migrations are required even on fresh DBs                                 | Persistence                      |
| [L-009](L-009-concurrent-worktree-deadlock.md)                | Concurrent worktree commits deadlock; isolate `AGH_HOME` + ports                 | Workflow                         |
| [L-010](L-010-model-name-validation.md)                       | Non-existent model name silently breaks the entire batch                         | Workflow / CI                    |
| [L-011](L-011-fraco-test-coverage-pushback.md)                | "Fraco" test coverage is the most repeated pushback on generated `_tasks.md`     | Spec authoring                   |
| [L-012](L-012-techspec-prose-only-rework.md)                  | TechSpec without Go interface signatures triggers heavy review rework            | Spec authoring                   |
| [L-013](L-013-prd-must-not-name-implementation.md)            | PRD must not name frameworks, storage, error codes, or file formats              | Spec authoring                   |
| [L-014](L-014-sandbox-vocabulary-drift.md)                    | Runtime vocabulary must match public contracts                                   | Architecture / Product           |
| [L-015](L-015-native-provider-auth-boundary.md)               | Provider auth ownership must be explicit                                         | Architecture / Security          |
| [L-016](L-016-native-provider-qa-home-policy.md)              | Native provider QA must respect home policy                                      | Testing / Workflow               |
| [L-017](L-017-named-sse-listener-registration.md)             | Named SSE events require explicit `addEventListener` registration                | Frontend / SSE                   |
| [L-018](L-018-delegated-docs-runtime-truth-audit.md)          | Delegated docs lanes need a runtime-truth audit before acceptance                | Documentation                    |
| [L-019](L-019-diagnostic-data-outlives-primary-record.md)     | Diagnostic data must outlive its primary record when audit/replay matters        | Architecture / Persistence       |
| [L-020](L-020-dense-typed-records-need-pointer-boundaries.md) | Dense typed orchestration records need pointer boundaries                        | Architecture / Code style        |
| [L-021](L-021-schema-migration-identity-is-append-only.md)    | Schema migration identity is append-only                                         | Persistence                      |
| [L-022](L-022-eyebrow-canonical-source.md)                    | Eyebrow typography needs one canonical source                                    | Frontend / Design system         |
| [L-023](L-023-token-utility-canonical-form.md)                | Design tokens belong in `@theme`, not in `:root` aliased through `@theme inline` | Frontend / Design system / Build |
| [L-024](L-024-design-md-generated-tokens.md)                  | Generated design-token specs prevent documentation drift                         | Frontend / Design system / Docs  |
| [L-025](L-025-greenfield-hardcut-current-protocol-version.md) | On greenfield, hard-cut the current protocol version — don't bump to a new one   | Project posture / RFC discipline |
| [L-026](L-026-integration-claims-require-substrate-evidence.md) | Integration claims require substrate evidence, not symbol greps                 | Analysis / Architecture review   |
| [L-027](L-027-judge-platform-by-premise-not-reference-implementation.md) | Judge a platform against its premise, not its reference implementation | Architecture / Spec authoring    |
| [L-028](L-028-correction-is-not-license-for-opposite-extreme.md) | A correction is not a license for the opposite extreme                          | Decision process / Spec authoring |
| [L-029](L-029-qa-labs-must-tear-down-processes.md)            | QA labs must tear down processes, not just isolate them                          | Workflow / QA hygiene            |
| [L-030](L-030-dual-verify-capacity-collapse.md)               | Two concurrent `make verify` runs collapse the machine; serialize the gate       | Workflow / Build tooling         |
| [L-031](L-031-primitive-reuse-is-a-gate-not-prose.md)         | Primitive reuse is a gate, not prose — shadows of `@agh/ui` exports fail lint    | Frontend / Design system / Process |
| [L-032](L-032-prototype-fidelity-binds-visual-language-not-content.md) | Prototype fidelity binds visual language, not content                   | Frontend / Design system / Process |

## How to use

When designing a new feature or reviewing a PR, scan the index for adjacent classes. Each lesson cites specific files/incidents so you can verify whether it still applies.

When you discover a new lesson, add a numbered file (`L-NNN-kebab-case-title.md`) and update this index. Keep one lesson per file. Cite specific evidence (file path, commit, review issue, ledger entry).

## When NOT to add a lesson

- Don't add speculative warnings — only confirmed incidents.
- Don't duplicate CLAUDE.md or `docs/_memory/standing_directives.md` rules. Lessons explain _why_ a rule exists; rules go in their respective files.
- Don't capture one-off bug fixes that have no transferable insight. The lesson must generalize beyond the specific bug.
