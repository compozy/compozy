---
status: pending
parallelizable: true
blocked_by: ["1.0", "5.0"]
---

<task_context>
<domain>ops, docs</domain>
<type>documentation</type>
<scope>operations</scope>
<complexity>low</complexity>
<dependencies>compose, docs</dependencies>
<unblocks>—</unblocks>
</task_context>

# Task 10.0: Ops & docs updates (Redis 8, backups, guides)

## Overview

Upgrade dev compose to Redis 8.x, document AOF/RDB, backups, and rollouts; update README and add `docs/redis-persistence.md`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Compose: `REDIS_VERSION=8-alpine` (dev)
- Docs: retention policy, admin tools usage, persistence guidance
- Ensure clarity and brevity
</requirements>

## Subtasks

- [ ] 10.1 Bump compose to Redis 8.x and validate configs
- [ ] 10.2 Add docs for persistence, backups, retention, admin cmds
- [ ] 10.3 Update README quick start

## Sequencing

- Blocked by: 1.0, 5.0
- Parallelizable: Yes

## Implementation Details

See tech spec sections 39–56 (vectors), 9 (ops), and 17 (rollout).

### Relevant Files

- `cluster/docker-compose.yml`
- `docs/redis-persistence.md`
- `README.md`

## Success Criteria

- Docs merged; compose updated; tests/lints unaffected
