# Auth System – Technical Specification

## 1. Executive Summary

This document defines the technical design for the API-key Authentication System described in the PRD. The solution introduces a dedicated `engine/auth` domain providing:

- Key-based authentication middleware (Gin)
- Redis-backed key caching and rate limiting
- Postgres persistence for users and API keys managed via Goose migrations
- CLI sub-commands under `auth` for key/user operations
  The design follows Clean Architecture patterns, clean separation of domain/application/infrastructure layers, and all core project standards.

---

## 2. System Architecture

### 2.1 Domain Placement

```
engine/
  auth/          <-- NEW (domain logic, use cases, router)
    model/       (User, APIKey)
    uc/          (services/interfaces)
    router/      (HTTP handlers)
  infra/
    redis/       (existing client – reused)
    postgres/    (existing connection – migrations added)
```

All HTTP endpoints are mounted in the existing `infra/server` using a new router registration similar to other domains.

### 2.2 Component Overview

```
┌──────────────┐ 1. request  ┌────────────┐ 2. parse header  ┌────────┐
│ HTTP Client  │ ───────────▶│ Gin Router │──────────────────▶│ AuthMW │
└──────────────┘             └────────────┘                  │        │
                                                               │        │
                            4. Redis  ◀────────────────────────┘        │
                                         cache miss                   ▼
                                   ┌──────────────────┐ 5. load      ┌──────────┐
                                   │ Postgres (auth) │──────────────▶│ use case │
                                   └──────────────────┘               └──────────┘
```

- **AuthMW** – Gin middleware validating key, enforcing rate limit, injecting user context
- **Redis Cache** – key→user/role mapping (30 s TTL)
- **Rate Limiter** – Per-key token bucket (100 req/min) using existing `engine/infra/server` middleware.
- **Use-case Layer** – CRUD services for users and keys

---

## 3. Implementation Design

### 3.1 Data Models

```go
// engine/auth/model/user.go
package model

type Role string
const (
    RoleAdmin Role = "admin"
    RoleUser  Role = "user"
)

type User struct {
    ID        core.ID `db:"id,pk"`
    Email     string  `db:"email,unique"`
    Role      Role    `db:"role"`
    CreatedAt time.Time `db:"created_at"`
}

// engine/auth/model/apikey.go
package model

type APIKey struct {
    ID        core.ID  `db:"id,pk"`
    UserID    core.ID  `db:"user_id"`
    Hash      []byte   `db:"hash"`           // bcrypt-hashed key
    Prefix    string   `db:"prefix"`         // cpzy_
    CreatedAt time.Time `db:"created_at"`
    LastUsed  *time.Time `db:"last_used"`
}
```

### 3.2 Key Generation

```go
func GenerateKey() (plaintext string, hash []byte, err error) {
    raw := make([]byte, 32)               // 256-bit entropy
    if _, err = rand.Read(raw); err != nil { return }
    plaintext = "cpzy_" + base62.Encode(raw)
    hash, err = bcrypt.GenerateFromPassword([]byte(plaintext), bcrypt.DefaultCost)
    return
}
```

### 3.3 Authentication Middleware

```go
func AuthMiddleware(svc auth.UCService) gin.HandlerFunc {
    return func(c *gin.Context) {
        key := extract(c.Request.Header)
        if key == "" { c.JSON(401, ...); c.Abort(); return }
        user, err := svc.ValidateKey(c, key)
        if err != nil { c.JSON(401, ...); c.Abort(); return }
        // Rate limiting is handled by the existing server middleware, which will be
        // configured to rate limit on a per-key basis.
        ctx := userctx.WithUser(c.Request.Context(), user)
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

### 3.4 Use-case Interfaces

```go
// engine/auth/uc/service.go
package uc

type Service interface {
    ValidateKey(ctx context.Context, plaintext string) (*model.User, error)
    GenerateKey(ctx context.Context, userID core.ID) (string, error)
    RevokeKey(ctx context.Context, keyID core.ID) error
}
```

Infra implementations reside in `engine/auth/infra/{postgres,redis}`.

### 3.5 API Routes (Gin)

Registered under `/api/v0` in a new router module.

```go
auth := r.Group("/auth", authmw.AuthMiddleware(svc))
{
    auth.POST("/generate", h.GenerateKey)
    auth.GET("/keys", h.ListKeys)
    auth.DELETE("/keys/:id", h.RevokeKey)
}
admin := r.Group("/users", authmw.AdminOnly())
{
    admin.GET("", h.ListUsers)
    admin.POST("", h.CreateUser)
    // ...update/delete
}
```

### 3.6 Goose Migrations

```
-- 0001_create_users.sql
CREATE TABLE users (...);

-- 0002_create_api_keys.sql
CREATE TABLE api_keys (...);
CREATE INDEX idx_api_keys_hash ON api_keys(hash);
```

Files placed in `engine/auth/migrations/` and wired to existing Goose runner.

---

## 4. Integration Points

- **Redis** (existing pool) – caching.
- **Postgres** – new tables in default schema.
- **Rate Limiter** - The existing rate limit middleware in `engine/infra/server` will be configured for per-key limits.
- **Prometheus** – exported via `pkg/monitoring` middleware: `auth_requests_total`, `auth_failures_total`, `auth_latency_seconds`, `rate_limit_blocks_total`.
- **CLI** – extend `cli/root.go` with `auth` group using Cobra.

---

## 5. Impact Analysis

| Component             | Impact                                                 |
| --------------------- | ------------------------------------------------------ |
| infra/server          | New router registration, middleware chain order update |
| pkg/logger            | No change – middleware extracts logger from ctx        |
| rate-limit middleware | Updated to support per-key buckets.                    |
| Deployment            | No new infra services required.                        |

No breaking API changes; existing public endpoints will accept requests without keys until workflows are flagged _private_.

---

## 6. Testing Approach

- **Unit Tests** – key generation, hashing/validation, rate limiter edge cases. Use `stretchr/testify`.
- **Integration Tests** (test/helpers):
  - Spin-up Postgres + Redis containers.
  - Hit endpoints via `httptest` server verifying 401/200 paths, revocation, rate-limit.
  - Ensure logger and context propagation.
- Coverage target ≥ 80 % for `engine/auth` package.

---

## 7. Development Sequencing

1. Goose migrations + models
2. Use-case & repository implementations (Postgres)
3. Configure existing rate-limiter for per-key limits.
4. Middleware + router wiring
5. CLI commands
6. Metrics exposition
7. Tests (unit → integration)
8. Documentation updates (Swagger, docs site)

Parallel: rate-limit script + CLI can proceed once repository interfaces stabilise.

---

## 8. Monitoring & Observability

- **Counters**: `auth_requests_total{status="success|fail"}`
- **Histogram**: `auth_latency_seconds`
- **Counter**: `rate_limit_blocks_total`
  Dashboards: extend existing Grafana “API Monitoring” with auth panel; add alert on >1 % failure rate over 5 m.

---

## 9. Technical Considerations & Risks

- **Hash Algorithm** – bcrypt chosen for ubiquity and hardware cost; revisit argon2 if perf acceptable.
- **Entropy** – 256-bit random key renders brute-force infeasible (>2¹²⁸ attempts).
- **Redis Outage** – middleware falls back to Postgres lookup with local LRU (1000 entries) to avoid downtime.
- **Key Prefix Exposure** – prefix aids quick identification; only logged in debug mode with redaction.
- **Migration Ordering** – Goose version numbers must not clash; reserve 000x block for auth.
- **Compliance** – PII limited to email; ensure email column subject to existing GDPR deletion tooling.

---

_End of Tech Spec_
