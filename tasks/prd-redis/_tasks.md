# Redis-Only Persistence Migration — Implementation Task Summary

## Relevant Files

### Core Implementation Files

- `engine/infra/redis/keys.go` - Key namespaces, builders, and TTL helpers
- `engine/infra/redis/lua.go` - Lua script registration and invocations (upserts, index maintenance)
- `engine/infra/redis/authrepo.go` - Redis-backed `engine/auth` repository
- `engine/infra/redis/workflowrepo.go` - Redis-backed `engine/workflow` repository
- `engine/infra/redis/taskrepo.go` - Redis-backed `engine/task` repository
- `engine/infra/repo/provider.go` - Provider returning Redis-backed repos only
- `engine/infra/server/dependencies.go` - Wire Redis client and health check; label telemetry `store_driver=redis`

### Integration Points

- `pkg/config` - Use `config.FromContext(ctx)` for Redis configuration
- `engine/infra/cache` - Redis client interface, lock manager, miniredis testing patterns
- `engine/infra/monitoring` - Metrics/interceptors; add repo operation labels
- `cluster/docker-compose.yml` - Bump to Redis 8.x and remove app-postgresql
- `Makefile` - Remove Postgres migrate targets tied to app DB

### Documentation Files

- `docs/redis-persistence.md` - Persistence, AOF/RDB guidance, retention policy
- `README.md` - Quick start updates (Redis-only)

## Tasks

- [ ] 1.0 Create Redis infra scaffolding (keys, lua, helpers)
- [ ] 2.0 Implement Auth repository on Redis (behavior parity)
- [ ] 3.0 Implement Workflow repository on Redis (behavior parity)
- [ ] 4.0 Implement Task repository on Redis (behavior parity)
- [ ] 5.0 Replace provider and server wiring with Redis-only
- [ ] 6.0 Implement TTL/retention for terminal states
- [ ] 7.0 Add metrics/monitoring labels and health checks
- [ ] 8.0 Build admin reindex/check tools
- [ ] 9.0 Remove Postgres code, migrations, and tooling
- [ ] 10.0 Ops & docs updates (Redis 8, backups, guides)

## Execution Plan

- **Critical Path:** 1.0 → (2.0, 3.0, 4.0 in parallel) → 5.0 → 8.0 → 9.0
- **Parallel Track A:** 6.0 (after 4.0) → 7.0
- **Parallel Track B:** 10.0 (can start after 1.0; finalize after 5.0)

Notes:

- Keep `make lint` and `make test` green at every step. Use miniredis for unit tests.
- Use `logger.FromContext(ctx)` and `config.FromContext(ctx)` everywhere; avoid globals.
- Multi-key writes must be atomic via Lua scripts; expose a single internal Upsert per entity.
