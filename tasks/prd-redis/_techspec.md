Title: Compozy Tech Spec — Redis-Only Greenfield Replacement (Remove PostgreSQL)

Status: Draft for review
Owner: Platform/Infra
Last updated: 2025-09-20

Summary

- Goal: Perform a greenfield replacement of all PostgreSQL-backed persistence with Redis while preserving external behavior. Backwards compatibility and dual-backend support are not required (alpha phase). We will remove Postgres code, migrations, wiring, and make targets. Behavior and functionality must remain intact and `make lint`/`make test` must remain green.
- Scope: Auth (users, API keys), Workflow state, Task state. Non-scope: Temporal’s own DB (remains Postgres), existing Redis-based components (cache, rate-limit, memory store, resource store) which we will reuse patterns from.
- Constraints (must follow project rules):
  - Always use logger.FromContext(ctx) and config.FromContext(ctx); no global singletons; never use context.Background() in runtime code paths.
  - Avoid workarounds; implement proper solutions and update tests accordingly.
  - No backward compatibility is required in alpha; we will remove old Postgres code entirely. External behavior must be preserved and tests must pass.

Current State (As-Is)

- Postgres usage
  - Concrete driver and repos: engine/infra/postgres/\*
    - store.go: pgx pool initialization
    - dsn.go, migrations.go (goose), JSONB helpers, placeholders, scan helpers
    - authrepo.go: implements engine/auth/uc.Repository
    - workflowrepo.go: implements engine/workflow.Repository
    - taskrepo.go: implements engine/task.Repository, uses JSONB and advanced queries/indexes
  - Provider: engine/infra/repo/provider.go returns Postgres-backed repos only.
  - Server wiring: engine/infra/server/dependencies.go → postgres.NewStore + repo.NewProvider
  - Schema/migrations (engine/infra/postgres/migrations):
    - workflow_states: PK workflow_exec_id; indexes on status, workflow_id, created_at, updated_at.
    - task_states: PK task_exec_id; FKs to workflow_states and to parent task; many composite indexes for status, workflow, exec, agent/tool/action, hierarchy, created_at/updated_at.
    - users: PK id; unique(lower(email)); role enum.
    - api_keys: PK id; unique prefix; fingerprint unique; hash (bcrypt), last_used timestamp; index on user_id and created_at.

- Redis is already used and battle-tested in the codebase
  - Infra/cache adapter (engine/infra/cache/\*): client wrapper, lock manager, notifications; includes miniredis-based tests.
  - Memory store (engine/memory/store/redis.go): patterns for JSON storage, atomic Lua scripts, TTL handling.
  - Resource store (engine/resources/redis_store.go): SCAN + MGET batching, Pub/Sub, Lua GET+DEL.
  - Rate limiting middleware (engine/infra/server/middleware/ratelimit/\*) via Redis.

Vector Search & Versions

- Redis 8 introduces native vector similarity via Vector Sets (new core data type). Prior to Redis 8, vector search was available via Redis Stack (RediSearch >= 2.6).
- Our current dev image in cluster/docker-compose.yml is Redis 7.2 (image: `redis:${REDIS_VERSION:-7.2-alpine}`), which does not include native Vector Sets.
- The Go client `github.com/redis/go-redis/v9` can be used for Vector Sets via the generic command interface (e.g., `client.Do(ctx, "VADD", ...)`). Dedicated high-level helpers may land in future client versions; do not block on them.

Options to enable vectors now

- Upgrade to core Redis 8.x for native vectors:
  - In `cluster/docker-compose.yml`, set `REDIS_VERSION` to an 8.x tag (e.g., `8-alpine`).
  - Ensure persistence and config (AOF everysec, TLS, password) remain aligned after the bump.
- Alternatively, use Redis Stack (includes RediSearch) if vector search is needed on Redis 7.x:
  - Use the `redis/redis-stack-server` image and configure RediSearch indices for vector fields.

Recommendation

- Move to Redis 8.x as part of this migration so our persistence layer and future vector use cases share the same core runtime. Use go-redis v9 and issue Vector Set commands via `Do` until native helpers are available.

High-Level Design (To-Be)

1. Introduce Redis-backed repository implementations (behavior parity):
   - engine/auth/uc.Repository → engine/infra/redis/authrepo.go
   - engine/workflow.Repository → engine/infra/redis/workflowrepo.go
   - engine/task.Repository → engine/infra/redis/taskrepo.go
     All must depend on cache.RedisInterface (engine/infra/cache) to avoid hard-coupling to go-redis and to fit our test strategy with miniredis.

2. Replace the provider and server wiring (Redis-only):
   - engine/infra/repo/provider.go → depend solely on cache.RedisInterface. Provide `NewProvider(r cache.RedisInterface)` returning only Redis-backed repos.
   - engine/infra/server/dependencies.go → construct a Redis client from config.Redis and pass it to repo.Provider. Remove goose migration logic and Postgres health checks from startup; keep Redis health checks via PING.

3. Data modeling in Redis (keys, values, indexes)
   General guidance:
   - Store primary entities as JSON values under object keys; maintain secondary indexes explicitly. Prefer SET (string) with JSON payloads for objects; use SETs (SADD/SMEMBERS) or sorted sets (ZADD/ZSCORE/ZRANGE) for indexes. Use Lua for multi-key atomic updates where a write touches multiple keys.
   - Namespacing: prefix all keys with an explicit version and environment to support multi-tenant safety and future clustering. Recommended form: `compozy:v1:<env>:<domain>:...` and include a tenant slug when applicable: `{compozy:v1:<env>:<tenant>}:<domain>:...` (hash tag braces group tenant keys on a single slot in Redis Cluster).
   - Index correctness guardrails (required):
     - Single write funnel: expose only one internal Upsert API per entity; all writes must go through the corresponding Lua script.
     - Sentinel hash per object: maintain a tiny `:idx` HASH adjacent to each object containing only fields that indexes depend on (e.g., for tasks: `status`, `agent_id`, `tool_id`, `parent_id`, `workflow_exec_id`, `task_id`). Scripts read previous values from this hash to remove stale index memberships without JSON parsing.
     - Version field: persist a monotonically increasing `version` for every object and increment it on each write. Scripts can short-circuit when no indexed field changed and callers can detect lost-update races.

       3.1) Auth

   - User
     - Key: compozy:auth:user:<user_id> → JSON(User)
     - Index: compozy:auth:users (SET of user_ids) for ListUsers
     - Unique email mapping: compozy:auth:user_by_email:<lower_email> → <user_id> (SETNX on create)
   - API Key
     - Key: compozy:auth:apikey:<key_id> → JSON(APIKey)
     - Index: compozy:auth:keys_by_user:<user_id> (SET of key_ids)
     - Lookup by fingerprint (SHA-256 of bcrypt hash): compozy:auth:apikey_by_fp:<hex_fp> → <key_id> (SETNX on create)
     - Unique prefix mapping (if needed): compozy:auth:apikey_by_prefix:<prefix> → <key_id>
     - Update last_used: write JSON field and optionally mirror last_used in a ZSET for reporting if needed later.

       3.2) Workflow state

   - Object: compozy:wf:state:<workflow_exec_id> → JSON(workflow.State)
   - Index by workflow_id: compozy:wf:states_by_id:<workflow_id> (SET of workflow_exec_id)
   - Index by status: compozy:wf:states_by_id_status:<workflow_id>:<status> (SET)
   - Required sorted order: maintain compozy:wf:created_at (ZSET score=unix_ts, member=workflow_exec_id) to provide "latest first" listings without client-side sorts.

     3.3) Task state

   - Object: compozy:task:state:<task_exec_id> → JSON(task.State)
   - Index by workflow_exec: compozy:task:by_wfexec:<workflow_exec_id> (SET of task_exec_id)
   - By status in a workflow_exec: compozy:task:by_wfexec_status:<workflow_exec_id>:<status> (SET)
   - By agent/tool in a workflow_exec: compozy:task:by_wfexec_agent:<workflow_exec_id>:<agent_id> (SET); compozy:task:by_wfexec_tool:<workflow_exec_id>:<tool_id> (SET)
   - By task_id in a workflow_exec: compozy:task:by_wfexec_taskid:<workflow_exec_id>:<task_id> (SET of task_exec_id)
   - Parent-child mapping:
     - compozy:task:children:<parent_state_id> (SET of child task_exec_ids)
     - compozy:task:child_by_taskid:<parent_state_id> (HASH task_id → task_exec_id) for GetChildByTaskID
   - Required sorted views (created_at desc): ZSETs per parent and per workflow for fast "latest first" queries, e.g. compozy:task:by_parent_created:<parent_state_id> and compozy:task:by_wfexec_created:<workflow_exec_id> (ZSET score=created_unix_ts, member=task_exec_id).

   Notes:
   - We do not rely on RedisJSON; plain JSON values and explicit indexes keep dependencies minimal and align with existing patterns in engine/resources and engine/memory.
   - Consistency is maintained by updating the object and all affected index keys atomically via Lua scripts or WATCH+TxPipelined (see Transactions section).

4. Transactions, locking, and atomicity
   - Optimistic transactions: Use go-redis WATCH with TxPipelined to implement Repository.WithTransaction where callers perform multiple operations that must be consistent. WATCH the primary object keys involved; inside the Tx pipeline, update the object and all indexes. If watched keys change, retry with jittered backoff.
   - Row-level “for update”: Implement task.Repository.GetStateForUpdate via Redis distributed locking using the existing lock manager (engine/infra/cache/lock_manager.go) with a lock key compozy:lock:task:<task_exec_id>. Standardize lock TTL to 5s, acquisition timeout to 50ms with jittered backoff (cap 500ms), and emit metrics for lock_wait, lock_acquired, lock_timeout, and hold_duration.
   - Lua for multi-key atomic updates: For single upsert operations that touch an object and several index keys (e.g., UpsertState), use EVAL with a script that (a) writes object (SET), (b) SADD/ZADD index entries, (c) removes stale index entries based on previous values read from the `:idx` HASH, and (d) updates `version` and the sentinel hash. This ensures atomicity without extra round-trips and avoids WATCH retries in hot paths.

   References used to validate patterns (2025):
   - go-redis v9 supports WATCH/TxPipelined for optimistic transactions and Lua via Eval. See Redis client docs on pipelines/transactions and Redis optimistic locking.

5. Query mapping (from Postgres to Redis)
   Repository methods and proposed Redis operations:
   - Auth
     - CreateUser: SETNX compozy:auth:user_by_email:<lower_email> → user_id; SET compozy:auth:user:<id> JSON; SADD compozy:auth:users <id> (Lua for atomic multi-key create).
     - GetUserByID: GET compozy:auth:user:<id>.
     - GetUserByEmail: GET compozy:auth:user_by_email:<lower_email> → id; then GET user object.
     - ListUsers: SMEMBERS compozy:auth:users; MGET users in batches.
     - UpdateUser: If email changed, SETNX new email mapping, DEL old mapping; update object (Lua script to ensure atomicity and uniqueness).
     - DeleteUser: DEL user object; SREM from users set; clean up related API key indexes.
     - API Keys: CreateAPIKey (SETNX fp mapping; SET object; SADD keys_by_user), GetAPIKeyByHash (GET mapping then GET object by id), ListAPIKeysByUserID (SMEMBERS → MGET), UpdateAPIKeyLastUsed (JSON rewrite via SET of full object or HSET of a field if we switch to HASH), DeleteAPIKey (DEL object; SREM; DEL mappings).

   - Workflow
     - UpsertState: SET wf object; SADD compozy:wf:states_by_id:<workflow_id>; SADD states_by_id_status:<workflow_id>:<status>.
     - UpdateStatus: Move membership between per-status sets (SREM old; SADD new) and update object; do it atomically via Lua.
     - GetState: GET object by workflow_exec_id; to include tasks map, perform a follow-up fetch of tasks for that exec (SMEMBERS compozy:task:by_wfexec:<exec> → MGET) if caller expects enriched state (matching current Postgres behavior in repo).
     - GetStateByID / GetStateByTaskID / GetStateByAgentID / GetStateByToolID: Traverse respective indexes (e.g., compozy:task:by_wfexec_taskid, by_wfexec_agent, by_wfexec_tool) to locate tasks, then bind back to a workflow state.

   - Task
     - UpsertState: SET object; ensure membership in compozy:task:by_wfexec:<exec>; add to status/agent/tool indexes if present; update parent indexes if ParentStateID not nil; maintain created_at sorted ZSET if needed for ordered queries.
     - GetState: GET object.
     - WithTransaction: Implement via WATCH on compozy:task:state:<id> (and any other keys the closure plans to touch), then TxPipelined; or execute the closure under a per-key lock (lock manager) and use Lua where possible.
     - GetStateForUpdate: Acquire lock key compozy:lock:task:<task_exec_id>; read object; caller performs operations; release lock.
     - ListTasksInWorkflow: SMEMBERS compozy:task:by_wfexec:<exec> → MGET task objects.
     - ListTasksByStatus/Agent/Tool: SMEMBERS per-index set → MGET.
     - ListChildren/GetChildByTaskID/GetTaskTree: use children set and child_by_taskid hash; for deep trees, iterative traversal from the sets.
     - ListChildrenOutputs: iterate children set, filter for Output != nil (can maintain a helper set compozy:task:children_with_output:<parent> if needed for performance).
     - GetProgressInfo: reuse indexes and/or maintain aggregates (optional phase 2) or compute on demand.

6. Serialization & schema
   - Store the Go structs as canonical JSON using existing StableJSONBytes where applicable. Keep timestamps as RFC3339 or unix seconds consistently; choose unix seconds for ZSET scores.
   - Keep core.ID values as strings.
   - Add `version` (int) to each persisted object and increment on each successful write.

7. Error handling and observability
   - Always attach logger.FromContext(ctx). On Redis failures, include key type not sensitive values (e.g., log key kind like "apikey_fp" not the raw fingerprint) to avoid leaking secrets in logs.
   - Return domain errors consistent with current repos (e.g., store.ErrWorkflowNotFound). Map redis.Nil to ErrNotFound where appropriate.

8. Configuration & wiring
   - Use existing config.Redis fields (pkg/config/config.go) and cache.NewRedis/SetupCache to construct cache.RedisInterface.
   - Remove reliance on DatabaseConfig for persistence paths. DatabaseConfig remains for Temporal and potential future use, but app store no longer uses it.
   - Update server setup (engine/infra/server/dependencies.go):
     - Replace postgres store building with Redis client creation + health check.
     - Instantiate repo.Provider with Redis client.
     - Update startup labels: store_driver=redis.

9. Operations
   - Docker Compose (cluster/docker-compose.yml):
     - Remove app-postgresql service and its volume; keep temporal-postgresql.
     - Ensure Redis runs with AOF enabled (appendonly yes) and a sane fsync policy (everysec) in dev; keep existing redis-dev.conf mount.
   - Backups: recommend AOF + periodic RDB snapshots for faster reload; include a short doc in /docs on redis persistence and backup strategy.
   - Security: enforce password, TLS as needed (config.Redis.TLSEnabled); consider ACLs in production.
   - Metrics: leverage existing OpenTelemetry and add counters around repo operations similar to Postgres code paths.
   - Retention policy & TTLs:
     - Terminal task states: apply TTL (configurable) with recommended range 7–30 days.
     - Workflow states: apply TTL (configurable) with recommended range 30–90 days.
     - Auth objects (users, API keys): no TTL.
     - Document AOF rewrite cadence and snapshot schedule; add alerts when keyspace size exceeds configured budgets.

10. Performance expectations

- Reads and writes should be significantly faster with Redis. Memory overhead increases due to duplicated secondary indexes; we accept this trade-off. For large cardinality sets, consider sharding index keys by prefix if needed.
- Use MGET and pipelining to batch I/O for list operations.

11. Test Strategy

- Unit and integration tests for new Redis repos under engine/infra/redis use miniredis (already present across the repo). Add helpers mirroring test/helpers/store_test_helpers.go for Redis.
- Remove Postgres test dependencies from affected tests; retain Temporal and other unrelated tests.
- Ensure race-free behavior using lock manager tests already present in engine/infra/cache.
- Keep existing behavior-focused tests in domains (engine/auth/uc, engine/task, engine/workflow) unchanged; only swap repo implementations beneath them.

12. Delivery Plan (Greenfield Cutover)

Phase A — Implement Redis-only persistence

- Add `engine/infra/redis` package with shared helpers (key builders, Lua scripts, JSON marshal helpers).
- Implement `NewAuthRepo(client)`, `NewTaskRepo(client)`, `NewWorkflowRepo(client)` returning Redis-backed repos.
- Replace provider and server wiring to use only Redis (no toggle). Ensure `make lint` and `make test` green.

Phase B — Remove Postgres

- Delete `engine/infra/postgres` and goose migrations.
- Remove app-postgresql service from docker-compose and DB-specific Makefile targets (Temporal’s DB remains).
- Drop pgx/pgxmock dependencies from `go.mod`/`go.sum`.

Phase C — Ops and docs

- Update dashboards, labels (`store_driver=redis`), and add admin reindex/check tools.
- Document persistence, backups, and retention. Ensure CI remains green.

13. Detailed Implementation Notes per Repository

- AuthRepo (Redis):
  - CreateUser(user): Lua script validates unique email via SETNX (normalize/lowercase email), writes object, SADD users; on error, roll back mappings. Maintain `compozy:auth:user:<id>:idx` with fields used by indexes and bump `version` on success.
  - GetUserByEmail: GET mapping then GET object.
  - UpdateUser(user): If email changed, Lua to check/set new mapping, del old mapping, update object atomically; update `:idx` and `version`.
  - CreateAPIKey: Lua for SETNX fp mapping, write object, SADD keys_by_user. Keep prefix uniqueness via a mapping if required by current API.
  - UpdateAPIKeyLastUsed: PATCH object; consider storing last_used in a HASH to avoid full JSON rewrites (post-flip optimization).

- WorkflowRepo (Redis):
  - UpsertState: Lua to set object + ensure index membership for (workflow_id) and (workflow_id,status); update `compozy:wf:state:<id>:idx` and `version`. Maintain membership in `compozy:wf:created_at` ZSET with score = created_unix_ts.
  - UpdateStatus: Lua that updates status sets (SREM from old, SADD to new) and rewrites object; use previous values from `:idx` to avoid stale membership.

- TaskRepo (Redis):
  - UpsertState: Lua that writes object and updates all affected indexes (wfexec, status, agent, tool, parent mappings). Use values stored in `compozy:task:state:<id>:idx` to remove stale memberships when TaskID, Status, AgentID, ToolID, ParentID change; refresh the sentinel hash and bump `version`.
  - WithTransaction: provide WATCH-based wrapper for callers that need multi-object ops; expose a closure with a repo bound to a local key prefix (or capture the tx client) so all operations are guaranteed to run under the same WATCH context.
  - GetStateForUpdate: acquire lock via lock manager; document that lock is required for cross-object consistency where WATCH is not used.
  - Ordering: maintain `compozy:task:by_parent_created:<parent_state_id>` and `compozy:task:by_wfexec_created:<workflow_exec_id>` ZSETs scored by created_unix_ts.

14. Code Changes Checklist (surgical paths)

- Add: engine/infra/redis/authrepo.go; engine/infra/redis/workflowrepo.go; engine/infra/redis/taskrepo.go; engine/infra/redis/keys.go; engine/infra/redis/lua.go
- Update: engine/infra/repo/provider.go (switch to RedisProvider)
- Update: engine/infra/server/dependencies.go (setupStore → Redis, store_driver label=redis)
- Update: go.mod (remove pgx deps at Phase 3)
- Remove (Phase 3): engine/infra/postgres/_; Makefile migrate-_ targets (or gate them behind Temporal only); cluster/docker-compose.yml app-postgresql service + volume
- Docs: add /docs/redis-persistence.md with AOF/RDB guidance; update README quick start.
- Admin tool: add `cmd/compozy-admin` with `redis reindex` (scan, diff, dry-run/repair) and `redis check-consistency` commands.

15. Risks & Mitigations

- Consistency: Multi-key updates can get out of sync if not done atomically → enforce Lua for writes that touch multiple keys; keep WATCH for complex cross-entity workflows; add sentinel `:idx` hashes + `version` fields per object and ship a reindex/consistency checker tool.
- Query surface: Some SQL queries map to multiple SCAN/SMEMBERS calls → design dedicated indexes to match hot paths (already listed). Avoid SCAN in critical request paths; rely on sets.
- Memory growth: Explicit indexes increase memory → apply TTLs to terminal states, add budgets and alerts, monitor with Redis INFO and keyspace scans; prune indexes where rarely used.
- Locking semantics: Redlock vs simple locks → we already ship a lock manager; for single Redis deployment in dev/alpha, SET NX with TTL is adequate. Re-evaluate when sharding/cluster is introduced.
- Durability: Enable AOF everysec; combine with RDB snapshots for faster recovery. Document trade-offs.

16. Open Questions

- (resolved) Ordering guarantees for task listings: ZSETs are required and implemented per parent and per workflow exec (see sections 3.2 and 3.3).
- Do we need additional search/indexing (e.g., by action_id beyond exact match)? If fuzzy search emerges, consider Redis Search, but avoid adding Redis Stack in alpha.

17. Rollout Plan

- Land Redis repos and Redis-only provider wiring (no flag).
- Ensure CI runs the whole test suite with miniredis for unit tests and dev Redis for integration as applicable.
- Remove Postgres code and targets; archive migrations in a /legacy folder only if required by policy.

18. Post-flip follow-ups (nice-to-haves)

- Memory budgeting doc: per-object size estimates and index fan-out; alerts at 60/80/90% memory.
- Hot index sharding plan: for very large sets (e.g., `task:by_wfexec:*`), shard by time buckets like `:d:YYYYMMDD` with union strategy if needed.
- API Key fast-path: store `last_used` as a small HASH to avoid full JSON rewrites.
- Extended property-based/chaos tests for write concurrency and index invariants.

Appendix A — Example Key Schema (Auth)

- compozy:auth:user:<id> → { id, email, role, created_at }
- compozy:auth:users → SET<id>
- compozy:auth:user_by_email:<lower_email> → <id>
- compozy:auth:apikey:<id> → { id, user_id, prefix, fingerprint, hash, created_at, last_used }
- compozy:auth:keys_by_user:<user_id> → SET<apikey_id>
- compozy:auth:apikey_by_fp:<hex_fp> → <id>

Appendix B — Example Lua Upsert (Task)
Pseudocode illustrating atomic multi-key upsert for task state and indexes:

```
-- KEYS[1]=task object key, KEYS[2]=by_wfexec set, KEYS[3]=by_wfexec_status set, ...
-- ARGV include JSON payload, status, agent, tool, parent, timestamps, etc.
local objKey    = KEYS[1]
local json      = ARGV[1]
local wfexecSet = KEYS[2]
local statusSet = KEYS[3]
-- (optionally) old status detection if the object exists: read and compare
-- write object
redis.call('SET', objKey, json)
-- ensure memberships
redis.call('SADD', wfexecSet, ARGV[2])         -- task_exec_id
redis.call('SADD', statusSet, ARGV[2])         -- task_exec_id
-- similarly SADD agent/tool sets if provided
return 'OK'
```

Appendix C — Testing Notes

- Use github.com/alicebob/miniredis/v2 in repo tests. Follow patterns in engine/infra/cache/\*\_test.go and engine/resources/redis_store_test.go.
- Avoid external Docker dependencies for unit tests; reserve integration runs for end-to-end validation.
- Add property-based tests that generate random upsert/delete sequences and assert object/index equivalence; include chaos tests with concurrent writers to validate sentinel-hash scripts and locking.

Appendix D — What we will remove at Phase 3

- Code: engine/infra/postgres/\*
- DB targets in Makefile: migrate-\*, reset-db (for app DB) — leave Temporal’s DB untouched.
- app-postgresql service in cluster/docker-compose.yml

Compliance Checklist

- Context usage: All new code paths accept ctx and use logger.FromContext(ctx) and config.FromContext(ctx).
- No globals: Use cache.SetupCache/NewRedis to obtain a client passed via provider wiring.
- Tests: Ensure make lint and make test pass on each phase before merge.
- Zen MCP: Run pre-change analysis for complex flows and post-change codereview; surface all recommendations/issues.

Appendix E — Admin reindex tool (skeleton)

- `compozy admin reindex --entity=task --dry-run` scans canonical object keys, recomputes expected index memberships, diffs, and can repair.
- `compozy admin check-consistency` verifies sentinel `:idx` hashes and reports drift.
- Run nightly in CI and before Phase 3 removal.
