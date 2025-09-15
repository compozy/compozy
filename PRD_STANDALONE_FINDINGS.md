# PRD Standalone – Current Gaps and Root‑Cause Findings

Owner: Compozy Platform
Date: 2025-09-15

## Summary

Standalone mode is only partially implemented. The HTTP server and storage subsystems (SQLite store, embedded MCP proxy, optional embedded Temporal) can run without external infra, but the Temporal worker still has a hard dependency on Redis. When Redis is absent, the server intentionally skips starting the worker, and all workflow endpoints return HTTP 503. Additionally, the embedded Temporal server is compiled behind a build tag (`embedded_temporal`), making runtime `mode=standalone` insufficient by itself.

This document explains why examples like `examples/weather` don’t execute in true standalone mode, and outlines what must change to meet the PRD requirement: “DON’T NEED ANY EXTERNAL SERVICE.”

## Reproduction

- Build/run: `make dev EXAMPLE=weather`.
- Do NOT run Redis/Valkey and do not set `REDIS_URL`.
- Server starts; workflow endpoints (e.g., POST start) respond with `503 Service Unavailable` and a JSON message like “worker unavailable”.

## Root Causes (code‑level)

1. Worker gated off when Redis is missing (server)

- File: `engine/infra/server/mod.go:maybeStartWorker`
- Behavior: In `standalone` mode, if `isRedisConfigured(cfg)` is false, the server logs “Redis not configured; starting server without worker in standalone”, sets `state.Worker = nil`, and continues. Downstream, workflow routes check `state.Worker == nil` and return 503.

2. Worker hard‑depends on Redis (worker)

- File: `engine/worker/mod.go:setupRedisAndConfig`
- Behavior: Always calls `cache.SetupCache(ctx, cacheConfig)` which creates a real Redis connection (`engine/infra/cache.NewRedis`). It also instantiates `services.NewRedisConfigStore` for storing task/config metadata. There is no in‑memory or embedded alternative. Even if we removed the server gate, the worker would fail to initialize without Redis.

3. Memory subsystem is Redis‑only

- File: `engine/memory/store/redis.go`
- Behavior: Memory store is implemented for Redis only. `memory.Manager` wiring expects a Redis lock manager and client, so memory features require Redis.

4. Rate limiting is not the blocker

- File: `engine/infra/server/mod.go:SetupRedisClient`
- Behavior: In `standalone`, if Redis isn’t configured, rate limiting falls back to an in‑memory limiter. This part does not force Redis.

5. Embedded Temporal requires a build tag

- Files: `engine/infra/server/temporal/embedded.go` (build tag `embedded_temporal`), `engine/infra/server/temporal/embedded_stub.go`
- Behavior: Without building with `-tags=embedded_temporal`, `StartEmbedded` returns: “embedded temporal server not enabled”. The Makefile adds this tag for `make dev` and `make test`, but runtime `mode=standalone` alone can’t enable it. This creates the perceived redundancy of “mode=standalone” + “tags=embedded_temporal”.

## Why “Redis is required” today

- The worker uses Redis for three roles:
  - Cache/locks/pubsub via `engine/infra/cache` (distributed cache & locking)
  - Task/config store via `engine/task/services/redis_store.go` (TTL + key ops)
  - Memory store via `engine/memory/store/redis.go` (append/trim lists, metadata)
- None of these paths provide an embedded or in‑memory implementation when `cfg.Mode == "standalone"`.
- The server therefore skips starting the worker to avoid immediate panic/fail, yielding 503s on workflow endpoints.

## Why do we need `tags=embedded_temporal`?

- Build‑time constraint: `embedded.go` (programmatic Temporal server + SQLite) is compiled only when the `embedded_temporal` tag is present. Otherwise, `embedded_stub.go` returns an error.
- Rationale (likely): keep heavy server dependencies out of default builds, faster CI, smaller binaries. But it makes `mode=standalone` insufficient by itself.

## Gaps versus PRD (“no external service”)

- Worker cannot operate without Redis → violates PRD.
- Memory subsystem has no non‑Redis backend → violates PRD.
- Config/ephemeral state uses Redis only → violates PRD.
- Embedded Temporal requires a special build tag instead of enabling automatically by mode → UX inconsistency.

## Code Evidence (verified)

- Server skips worker when Redis is absent in standalone: `engine/infra/server/mod.go:706–711`, specifically the log line at `engine/infra/server/mod.go:707`.
- Worker unconditionally wires Redis cache and config store: `engine/worker/mod.go:255–287` and helper at `engine/worker/mod.go:268–288`.
- Memory manager requires Redis lock manager and client: `engine/worker/mod.go:330–338`.
- Config store is Redis-only: constructor at `engine/task/services/redis_store.go:30–39`.
- Embedded Temporal stub returns error without build tag: `engine/infra/server/temporal/embedded_stub.go:1–20`.

Verified on commit 02a5c3b (September 15, 2025).

## Implementation Plan to Achieve True Standalone

1. Introduce an embedded cache and lock/notify layer for standalone

- Provide a SugarDB‑backed adapter in `engine/infra/sugardb` that implements:
  - `LockManager` (already partially available)
  - Pub/Sub notifications (already available)
  - Basic KV/TTL operations used by cache/config/memory
- Add `cache.SetupCacheStandalone(ctx)` returning `{Redis:nil, LockManager: sugarLock, Notification: sugarNotif}`.
- Switch worker to a generalized `setupCacheAndConfig(ctx)` that chooses SugarDB in standalone, Redis otherwise.

2. Config store: add SugarDB implementation

- New file: `engine/task/services/sugar_store.go` implementing `services.ConfigStore` using SugarDB. Must support:
  - `Save/Get/Delete` with TTL semantics (extend TTL on Get like current Redis path)
  - Metadata CRUD with namespaced keys
  - Test suite mirroring `redis_store.go` behavior

3. Memory store: add non‑Redis backend

- New file: `engine/memory/store/sugar.go` implementing the same semantics as Redis store:
  - Append/trim with metadata (message_count/token_count) and TTL operations
  - Read paginated, count, replace, metadata hash ops
- Update `memory.Manager` wiring to select Sugar store + Sugar lock manager when `mode=standalone`.

4. Remove the “no Redis → skip worker” gate for standalone

- In `maybeStartWorker`, when `cfg.Mode==standalone` and Redis is not configured, initialize the SugarDB‑based cache/config/memory and start the worker.
- Keep existing Temporal reachability check; rely on embedded server when enabled.

5. Build‑tag strategy for Temporal

- Always ship binaries with embedded Temporal enabled. Keep the code, remove the build tag, and conditionally start it only when `mode==standalone`.

6. Docs, Examples, and Tooling

- Update quickstart to reflect true standalone (no Redis needed).
- Add a health page explaining which backends are active (SQLite/SugarDB/Temporal embedded).
- Example `examples/weather`: clearly state expected behavior with/without Redis.

## Acceptance Criteria

- `make dev EXAMPLE=weather` with no Redis:
  - Worker starts; POST workflow returns 202 and executes.
  - Memory features work (append/read/ttl) using SugarDB.
  - Rate limiting remains in‑memory, as today.
  - Embedded Temporal starts automatically by `mode=standalone` (either via always‑on build, default tag, or helper binary).

## Risks & Trade‑offs

- SugarDB semantics vs Redis scripts (atomic append+trim+metadata): must be replicated carefully. We can port the Redis Lua logic into transactional SugarDB ops.
- Performance: single‑process KV is fine for dev; ensure production builds still default to Redis/Postgres paths.
- Complexity: Two backends doubles test surface; mitigate via shared interface tests.

## Zen MCP Findings (Tracer/Debug)

- Trace summary (dependencies mode):
  - Incoming: server bootstrap calls `maybeStartWorker` which, in `standalone` mode, returns early when `!isRedisConfigured(cfg)`; see `engine/infra/server/mod.go:706–711`.
  - Outgoing: when the worker is created, `setupWorkerCore` calls `setupRedisAndConfig` which creates a Redis cache and `NewRedisConfigStore`; see `engine/worker/mod.go:255–287` and `engine/worker/mod.go:268–288`.
  - Memory: `setupMemoryManager` expects `BaseLockManager` and `BaseRedisClient` from the Redis cache; see `engine/worker/mod.go:330–338`.
  - Temporal: without `embedded_temporal` build tag, `StartEmbedded` is a stub that returns an error; see `engine/infra/server/temporal/embedded_stub.go:14–16`.
- Debug conclusion: Behavior matches design—no fallback path for cache/config/memory in standalone, and embedded Temporal is compile‑time gated. Implementing SugarDB adapters and removing the build‑tag requirement are necessary to meet the PRD.

Note: Tracing executed with Zen MCP (Gemini 2.5 Pro) on September 15, 2025.

## Why `tags=embedded_temporal` is redundant from a product POV

- Users expect “standalone” to be a runtime switch. Requiring a compile‑time tag violates that expectation. We should either always build the embedded server or ensure our distributed binaries include it by default and document it clearly.

---

### Appendix: Immediate Low‑Effort Improvements

- Keep 503s but return a distinct error code like `WORKER_UNAVAILABLE`.
- Register worker‑dependent routes only when the worker is available (or add a “require‑worker” middleware) to reduce confusion.
- Log a one‑time hint when running in standalone with no Redis explaining that this is a known gap and suggesting `EXPERIMENTAL_STANDALONE_SUGARDB=1` once implemented.

### External Library Verification (Context7/Perplexity)

- SugarDB supports core primitives we intend to use in standalone: `SET` with options (including SETNX/expiration), `MGET`, `DEL`, hashes, sets, zsets, pub/sub, and `KEYS` for key enumeration. See upstream docs for Generic/Hash/Set command sets and Go embedded APIs (echovault/sugardb). Where Redis `SCAN` parity is needed, maintain an index (e.g., set of MCP names) as implemented in `pkg/mcp-proxy/sugardb_storage.go`.
- Embedded Temporal: Our code uses an embedded server behind a build tag (`embedded_temporal`); build tooling (`make dev`, `make test`) already enables the tag. Align binaries and mode behavior to remove the runtime/build split for end users.
