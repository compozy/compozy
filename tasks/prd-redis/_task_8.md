---
status: pending
parallelizable: false
blocked_by: ["2.0", "3.0", "4.0", "5.0", "6.0", "7.0"]
---

<task_context>
<domain>cmd/compozy-admin</domain>
<type>implementation</type>
<scope>tooling</scope>
<complexity>medium</complexity>
<dependencies>redis, repos</dependencies>
<unblocks>9.0</unblocks>
</task_context>

# Task 8.0: Build admin reindex/check tools

## Overview

Provide admin CLI to scan canonical objects, recompute expected index memberships, diff, and optionally repair; include consistency checks for sentinel `:idx` hashes.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Commands: `redis reindex` (dry-run/repair), `redis check-consistency`
- Safe logging; no sensitive data
- Tests with miniredis
</requirements>

## Subtasks

- [ ] 8.1 Create `cmd/compozy-admin` skeleton with Redis wiring
- [ ] 8.2 Implement reindex/check commands
- [ ] 8.3 Tests for diff/repair flows

## Sequencing

- Blocked by: 2.0, 3.0, 4.0, 5.0, 6.0, 7.0
- Unblocks: 9.0
- Parallelizable: No

## Implementation Details

See PRD R13 and tech spec Appendix E.

### Relevant Files

- `cmd/compozy-admin/*`

### Dependent Files

- `engine/infra/redis/*`

## Success Criteria

- CLI works against miniredis/dev Redis; tests and lints green
