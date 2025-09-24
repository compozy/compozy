---
status: pending
parallelizable: false
blocked_by: ["8.0"]
---

<task_context>
<domain>repo, infra, ops</domain>
<type>implementation</type>
<scope>cleanup</scope>
<complexity>medium</complexity>
<dependencies>git, build, compose</dependencies>
<unblocks>—</unblocks>
</task_context>

# Task 9.0: Remove Postgres code, migrations, and tooling

## Overview

Delete `engine/infra/postgres/*`, goose migrations, Makefile targets for app DB, and app-postgresql service from compose; keep Temporal’s DB intact.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- No backward compatibility required
- Ensure CI stays green; remove pgx deps from go.mod/go.sum
- Update scripts accordingly
</requirements>

## Subtasks

- [ ] 9.1 Remove code and migrations
- [ ] 9.2 Update Makefile and scripts
- [ ] 9.3 Update compose; verify Temporal DB remains
- [ ] 9.4 Prune go.mod/go.sum

## Sequencing

- Blocked by: 8.0
- Parallelizable: No

## Implementation Details

See tech spec sections 12 (Phase B) and 14 (checklist).

### Relevant Files

- `engine/infra/postgres/*`
- `Makefile`
- `cluster/docker-compose.yml`
- `go.mod`, `go.sum`

## Success Criteria

- No Postgres code remains; build/test/lint green
