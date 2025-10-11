## markdown

## status: completed

<task_context>
<domain>engine/llm/providers</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 1.0: Provider Registry & Capability Layer

## Overview

Refactor the provider factory into a capability-aware registry, isolate provider-specific quirks inside adapters, and expose routing hooks required for future fallback strategies.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- Introduce a central `Provider` interface with capability discovery (`SupportsStructuredOutput`, streaming, etc.).
- Replace the switch-based factory with a registry that supports provider self-registration.
- Move provider-specific hacks (e.g., Groq JSON handling) into provider adapters.
- Implement minimal routing hook to enable fallback sequencing without modifying the loop core yet.
- Maintain strict context usage via `logger.FromContext(ctx)` and `config.FromContext(ctx)`.
</requirements>

## Subtasks

- [x] 1.1 Define provider interfaces, capability structs, and registry APIs.
- [x] 1.2 Migrate existing providers (OpenAI, Anthropic, Groq, Google, XAI, DeepSeek, Ollama, Mock) to self-registering adapters encapsulating quirks.
- [x] 1.3 Update orchestrator/service wiring to resolve providers via the registry and remove inline conditionals.
- [x] 1.4 Add routing hook for fallback lists and document registration expectations.
- [x] 1.5 Implement unit tests covering registry lookup, duplicate registration guardrails, and provider capability flags.

## Implementation Details

- PRD §A “Provider Abstraction” highlights the need for a `ProviderRegistry` and encapsulation of provider quirks.
- Ensure the registry returns telemetry-wrapped providers when applicable to align with later tasks.
- Guard against nil contexts by inheriting only from caller-provided `ctx`.

### Relevant Files

- `engine/llm/adapter/providers.go`
- `engine/llm/adapter/provider_registry.go` (new)
- `engine/llm/service.go`
- `engine/llm/orchestrator/loop.go`

### Dependent Files

- `pkg/config/config.go`
- `engine/llm/orchestrator/request_builder.go`

## Deliverables

- Capability-aware provider registry with self-registering adapters.
- Updated orchestrator/service code consuming registry outputs with no residual provider-specific branching.
- Documentation snippet (in-code or README) describing registration and routing hook usage.

## Tests

- Unit tests mapped from PRD test strategy:
- [x] Routing fallback: provider A failure → provider B success.
- [x] Capability detection toggles structured-output mode when advertised.
- [x] Registry rejects duplicate provider keys and returns descriptive errors.

## Success Criteria

- All providers load via registry without switch statements.
- Structured-output capability is discoverable per provider.
- Fallback hook supports deterministic provider ordering for subsequent tasks.
- `make fmt && make lint && make test` pass.
