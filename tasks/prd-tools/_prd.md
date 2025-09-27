# Product Requirements Document (PRD) Template

## Overview

Compozy needs to retire the Bun-executed default tool suite and replace it with native Go implementations that are always available under the `cp__` prefix. Agent builders currently pay a cold-start penalty and face inconsistent error semantics because each tool invocation spins up a Bun worker. Platform operators also maintain duplicated dependency chains (Go + Bun) and fragmented security policies. The native tool initiative delivers lower-latency, deterministic tooling, consolidates security enforcement, and simplifies distribution for all workflows that depend on the default tools.

## Goals

- Reduce median execution latency for default tools (read, write, list, grep, exec, fetch) by at least 35% and 95th-percentile latency by 25% compared to the Bun baseline measured in sprint N-1.
- Eliminate Bun runtime dependency from the default toolkit, leaving Go as the only required runtime for built-in tools while preserving 100% functional parity with existing tool capabilities.
- Improve operational reliability by cutting tool-related incident volume (alerts, pagers, SLO breaches) by 50% within one month of release through centralized security validation, observability, and error cataloging.
- Achieve 90% positive developer-experience scores (internal survey) regarding default tool clarity and diagnostics during the first post-launch feedback cycle.

## User Stories

- As an agent builder, I want native cp\_\_ tools to respond within predictable latency budgets so that my workflows do not stall while the platform spins up a Bun process.
- As a platform operator, I want a single observability and logging surface for native tools so that incident response no longer requires correlating Go logs with Bun console output.
- As a security reviewer, I want centralized allowlist enforcement and filesystem sandboxing so that executing cp\_\_ tools cannot escape the configured project boundary or spawn arbitrary processes.
- As a documentation consumer, I want consistent CLI and docs references to cp\_\_ tools so that I can onboard new team members without explaining two separate tool ecosystems.

## Core Features

### Built-in Tool Registration and Availability

- Replace Bun-backed defaults with Go-native implementations packaged inside the core service.
- Ensure cp** tools auto-register during `llm.NewService` startup before custom tools load, while reserving the `cp**\*` namespace.

Functional Requirements

1. The system MUST register all cp** tools (`cp**read_file`, `cp**write_file`, `cp**delete_file`, `cp**list_dir`, `cp**grep`, `cp**exec`, `cp**fetch`) via a new builtin registry before any runtime-provided tools load.
2. The system MUST expose a kill-switch configuration flag (`native_tools.enabled`) that can disable native cp\_\_ tools and fall back to Bun implementations within five minutes without redeploying binaries.

### Filesystem Tooling (Read, Write, Delete, List, Grep)

- Provide secure path handling, root sandboxing, and binary-safety heuristics for filesystem interactions.

Functional Requirements

3. `cp__read_file` MUST resolve all input paths against `config.NativeTools.RootDir`, reject any path escaping that root, and return UTF-8 validated content with file metadata.
4. `cp__write_file` MUST create parent directories atomically (`os.MkdirAll`), enforce POSIX-safe file modes, and refuse to follow symlinks for write targets.
5. `cp__delete_file` MUST block deletions outside the configured root and reject directories unless an explicit recursive flag is provided.
6. `cp__list_dir` MUST support filtering (files/directories), pagination, and stop traversal after 10,000 entries or when the request context is canceled, whichever comes first.
7. `cp__grep` MUST stream results with limits (`maxResults`, `maxFilesVisited`, `maxFileBytes`), skipping files that contain NULL bytes within the first 8KiB or exceed the configured byte limit.

### Command Execution Tooling

- Preserve the existing allowlist model while eliminating shell injection vectors.

Functional Requirements

8. `cp__exec` MUST execute commands using `execabs.CommandContext` on Unix and an absolute-path validated Windows fallback, enforcing per-command argument schemas and rejecting any invocation exceeding configured argument counts or wall-clock timeouts.
9. Project-level configuration MUST append (not replace) default allowlisted commands, and the system MUST emit structured errors (`CommandNotAllowed`) when a command is blocked.

### Network Fetch Tooling

- Provide controlled outbound HTTP access with strong defaults.

Functional Requirements

10. `cp__fetch` MUST restrict outbound methods to a safe allowlist (GET, POST, PUT, PATCH, DELETE, HEAD), enforce a default 5-second timeout, cap response bodies at 2 MiB, limit redirects to five, and return structured telemetry (status code, latency, body truncation flag).

### Observability and Error Catalog

- Deliver consistent logs and metrics for cp\_\_ tool execution.

Functional Requirements

11. All cp\_\_ tools MUST emit structured logs with `tool_id`, `request_id`, `exit_code` (where applicable), and truncated stderr (1 KiB cap) while incrementing success/error Prometheus counters.
12. The platform MUST expose canonical error codes (`InvalidArgument`, `PermissionDenied`, `FileNotFound`, `CommandNotAllowed`, `Internal`) for every tool so that downstream orchestration can branch on failure states.

### Documentation and Developer Experience

- Update documentation, CLI scaffolding, and examples to promote cp\_\_ tool usage exclusively.

Functional Requirements

13. Official documentation, templates, and CLI scaffolding MUST reference only cp\_\_ tool identifiers and provide migration guidance for projects still using `@compozy/tool-*` imports.
14. Developer onboarding materials MUST include a troubleshooting guide covering sandbox violations, allowlist errors, and HTTP limit breaches with mapped error codes.

## User Experience

Primary Persona: Agent builders who configure workflows via YAML and expect deterministic tool execution. They interact through configuration files, CLI tooling, and automation pipelines.

Secondary Personas: Platform operators monitoring runtime health, and security/compliance reviewers verifying guardrails.

Key Flows

- Agent builder selects cp\_\_ tools in configuration and executes workflow; expects reduced latency, consistent error payloads, and detailed logs accessible through existing observability dashboards.
- Operator triages tool failures via unified metrics/logs without correlating Bun worker IDs.
- Security reviewer audits allowlists and filesystem boundaries through documented configuration knobs.

UX Considerations

- Ensure CLI error messages surface canonical error codes and human-readable guidance (e.g., “CommandNotAllowed: extend config.native_tools.exec.append_allowlist to permit `jq`”).
- Provide copy-paste-ready config snippets in docs with accessible language (plain descriptions, no jargon) and screen-reader friendly tables.
- Maintain consistent naming (`cp__toolname`) across docs, CLI help, and API responses.
- Guarantee documentation meets contrast and heading structure requirements for WCAG 2.1 AA when rendered on docs site.

## High-Level Technical Constraints

- Filesystem operations must remain confined to the configured project root, rejecting symlink traversal and path escapes.
- Command execution is limited to absolute-path allowlisted binaries with per-command argument schemas; shell expansion is prohibited.
- HTTP requests must honor default limits (5 s timeout, 2 MiB body cap, 5 redirects) with override hooks gated by configuration and audit logging.
- Observability must integrate with existing Compozy telemetry (Prometheus metrics, structured logging) without introducing new infrastructure dependencies.
- Cross-platform support: Linux and macOS must ship full functionality; Windows builds require the validated absolute-path fallback for `exec`.
- Configuration access must use `config.FromContext(ctx)` and logging must use `logger.FromContext(ctx)` in all code paths to comply with project standards.

## Non-Goals (Out of Scope)

- Migrating or rewriting user-defined custom tools that still rely on the Bun runtime; these remain unaffected.
- Building a long-lived Bun worker pool or optimizing Bun performance; the initiative removes Bun for default tools entirely.
- Introducing new tool categories beyond the existing default set (no new network or database tools in this scope).
- Reworking the LLM runtime orchestration or planner logic beyond registering the builtin tool set.
- Delivering a multi-phase rollout; this release lands the complete cp\_\_ tool suite in a single deployment, with the kill switch serving only as an emergency off-ramp.

## Phased Rollout Plan

- **MVP / General Availability:** Deliver the full cp\_\_ native tool suite enabled by default with the kill-switch configuration available for emergency rollback. No staged customer phases are planned due to the greenfield mandate.
- **Phase 2:** Not planned. Any future enhancements (e.g., streaming responses, concurrent execution) will be scoped separately.
- **Phase 3:** Not planned.

Success criteria for the single release: latency and reliability goals defined above are met in production, documentation is updated, and the Bun dependency is removed from release artifacts.

## Success Metrics

- Median cp\_\_ tool execution latency ≤ 150 ms; 95th percentile ≤ 250 ms under representative workloads.
- Tool-related production incidents (alerts, on-call pages) reduced by 50% within 30 days of launch compared to the previous quarter.
- 100% of Compozy-managed projects use cp\_\_ tool identifiers within two weeks; legacy `@compozy/tool-*` packages are removed from release artifacts.
- Developer-experience survey returns ≥ 90% “satisfied” or better responses on debugging clarity and documentation for default tools.
- Security review confirms zero critical findings related to path traversal or command injection in the new toolchain prior to enabling the feature flag in production.

## Risks and Mitigations

- **Parity Gaps:** Missing feature parity between cp\_\_ tools and Bun versions could break automations. _Mitigation:_ Maintain parity checklist, add regression tests mirroring Bun behavior, retain kill switch for rapid rollback.
- **Configuration Misuse:** Incorrect root directory or allowlist overrides may unintentionally widen access. _Mitigation:_ Ship safe defaults, provide linting/validation warnings, document configuration examples, and create monitoring alerts for configuration changes.
- **Documentation Drift:** Docs or templates might still reference Bun tools. _Mitigation:_ Assign doc ownership, add CI checks scanning for `@compozy/tool-` strings after launch, and run doc review with developer relations.
- **Operational Unknowns:** Lack of Bun fallback in production may expose undiscovered edge cases. _Mitigation:_ Exercise kill switch in pre-production, capture load/perf benchmarks, and prepare incident runbook for reverting to Bun if required.

## Open Questions

- Who owns long-term maintenance of the cp\_\_ documentation set and ensures updates stay synchronized with future enhancements?
- What baseline metrics will we capture pre-launch for latency and incident counts to verify the success criteria post-launch?
- Do we need additional guardrails for projects that intentionally operate outside the default root directory (e.g., multi-repo setups), or is a single-root policy sufficient?

## Appendix

- Reference: `tasks/prd-tools/_techspec.md` – Compozy Native Tool Migration Tech Spec.
- Benchmarks: Collect baseline Bun tool latency and error data from current observability dashboards prior to implementation.
- Comparable migrations: Kubernetes `kubectl` native plugins, HashiCorp Terraform CLI Go-native tooling, AWS CLI v2 rewrite case studies.
