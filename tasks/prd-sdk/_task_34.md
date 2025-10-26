## status: pending

<task_context>
<domain>sdk/memory</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>sdk/memory/config.go, engine/memory</dependencies>
</task_context>

# Task 34.0: Memory: Privacy + Expiration (S)

## Overview

Extend memory ConfigBuilder with privacy scope and expiration configuration.

<critical>
- **ALWAYS READ** @.cursor/rules/critical-validation.mdc before start
- **ALWAYS READ** tasks/prd-sdk/02-architecture.md and tasks/prd-sdk/03-sdk-entities.md
</critical>

<requirements>
- Privacy scope configuration (global, user, session)
- Expiration duration support
- Context-first validation
- Error accumulation
</requirements>

## Subtasks

- [ ] 34.1 Add WithPrivacy(privacy memory.PrivacyScope) method
- [ ] 34.2 Add WithExpiration(duration time.Duration) method
- [ ] 34.3 Update Build(ctx) validation for privacy/expiration
- [ ] 34.4 Add unit tests for privacy and expiration

## Implementation Details

Reference from 03-sdk-entities.md section 7.1:

```go
// Privacy and security
func (b *ConfigBuilder) WithPrivacy(privacy memory.PrivacyScope) *ConfigBuilder
func (b *ConfigBuilder) WithExpiration(duration time.Duration) *ConfigBuilder
```

Engine privacy scopes from engine/memory:
- PrivacyGlobalScope (shared across all users)
- PrivacyUserScope (isolated per user)
- PrivacySessionScope (isolated per session)

Example from architecture:
```go
memory.New("customer-support").
    WithPrivacy(memory.PrivacyUserScope).
    WithExpiration(24 * time.Hour)
```

### Relevant Files

- `sdk/memory/config.go` (extend existing)
- `engine/memory/types.go` (privacy scope types)

### Dependent Files

- Task 31.0 output (memory ConfigBuilder base)
- Future memory examples

## Deliverables

- Privacy scope method in ConfigBuilder
- Expiration duration method in ConfigBuilder
- Validation for privacy/expiration settings
- Updated package documentation

## Tests

Unit tests mapped from `_tests.md`:

- [ ] WithPrivacy sets privacy scope correctly
- [ ] WithExpiration sets expiration duration
- [ ] Build(ctx) validates privacy scope values
- [ ] Build(ctx) validates expiration > 0
- [ ] Error cases: invalid privacy scope, negative expiration
- [ ] Edge cases: zero expiration (never expire)

## Success Criteria

- Privacy and expiration methods follow builder pattern
- All unit tests pass
- make lint and make test pass
- Privacy/expiration ready for advanced memory configs
