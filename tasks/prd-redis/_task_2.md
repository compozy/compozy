---
status: pending
parallelizable: true
blocked_by: ["1.0"]
---

<task_context>
<domain>engine/auth</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>redis, lua, cache</dependencies>
<unblocks>5.0, 8.0</unblocks>
</task_context>

# Task 2.0: Implement Auth repository on Redis (behavior parity)

## Overview

Implement Redis-backed repo for Auth (users, API keys) with unique email and fingerprint mappings and required listings.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Preserve existing interfaces and behaviors
- Use Lua for multi-key writes (create/update/delete)
- No TTL for Auth objects
- Provide unit tests with miniredis
</requirements>

## Subtasks

- [ ] 2.1 Implement `authrepo.go` with CreateUser/Get/List/Update/Delete
- [ ] 2.2 Implement API key methods and uniqueness mappings
- [ ] 2.3 Add property tests for index consistency and uniqueness

## Sequencing

- Blocked by: 1.0
- Unblocks: 5.0, 8.0
- Parallelizable: Yes (independent from 3.0/4.0)

## Implementation Details

See tech spec sections 3.1, 5 (Auth mapping), 6 (serialization), 7 (errors).

### Relevant Files

- `engine/infra/redis/authrepo.go`

### Dependent Files

- `engine/infra/repo/provider.go`

## Success Criteria

- Behavior parity; tests and lints green; no TTL applied
