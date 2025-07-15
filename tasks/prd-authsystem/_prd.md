# Auth System – Product Requirements Document

## 1. Overview

### Problem Statement

Today all Compozy REST endpoints are publicly accessible, meaning any party can trigger workflows or query resources without restriction. This makes private usage impossible and exposes the platform to abuse.

### Value Proposition

Introducing an API-key based authentication layer ensures that:

- Only authorised users can execute workflows or access data.
- Workflows may be marked _private_ (key required) or _public_ (no key required) to retain flexibility during early adoption.
- Minimal friction: a single header‐based key keeps onboarding simple while we validate market demand.

### Target Users

- **Admin users** – bootstrap the workspace, create & manage users, rotate keys.
- **Regular users** – generate personal keys to run workflows via API or CLI.
- **Automation services / CI** – use keys for non-interactive calls.

---

## 2. Goals

| ID  | Goal                                       | Metric                                                                                  |
| --- | ------------------------------------------ | --------------------------------------------------------------------------------------- |
| G1  | Prevent unauthenticated workflow execution | ≥ 95 % of requests carry a valid key within 30 days of launch                           |
| G2  | Maintain low latency overhead              | Auth middleware adds ≤ 5 ms P99 to request path                                         |
| G3  | Block malicious traffic                    | ≥ 99 % of unauthorised requests rejected; 0 credential-stuffing successes first 30 days |
| G4  | Self-service key management                | 100 % of users can generate & revoke keys without admin assistance                      |

---

## 3. User Stories

1. **Generate Key** – _As a user_ I can call `POST /api/v0/auth/generate` to receive a new API key so that I can authenticate future requests.
2. **List Keys** – _As a user_ I can view all my active keys to audit integrations.
3. **Revoke Key** – _As a user_ I can revoke a compromised key so it can no longer access the API.
4. **Admin – Manage Users** – _As an admin_ I can create, read, update and delete users so that I control platform access.
5. **Admin – Manage Keys** – _As an admin_ I can revoke any user’s key for security response.
6. **Workflow Privacy** – _As a caller_ I must present a valid key when invoking a _private_ workflow, while _public_ workflows remain accessible without authentication.

Edge-case stories captured in Appendix.

---

## 4. Core Features

### 4.1 Authentication Middleware

- Inspect `Authorization: Bearer <API_KEY>` header.
- Validate key → user → role via in-memory cache backed by Postgres.
- Inject authenticated user into request context for downstream handlers.
- Reject with `401` on missing/invalid key.

### 4.2 API-Key Management Endpoints

| Method & Path                   | Auth  | Description                              |
| ------------------------------- | ----- | ---------------------------------------- |
| `POST /api/v0/auth/generate`    | user  | Generate new key (format `cpzy_<hash>`). |
| `GET /api/v0/auth/keys`         | user  | List caller’s keys.                      |
| `DELETE /api/v0/auth/keys/{id}` | user  | Revoke own key.                          |
| `GET /api/v0/users`             | admin | List users.                              |
| `POST /api/v0/users`            | admin | Create user (sets role).                 |
| `PATCH /api/v0/users/{id}`      | admin | Update user.                             |
| `DELETE /api/v0/users/{id}`     | admin | Delete user.                             |

### 4.3 Roles & Permissions

- `admin` – full user & key management.
- `user` – self-service key management, workflow execution.

### 4.4 CLI Support

- `compozy user create|list|delete`
- `compozy auth generate|revoke|list`

---

## 5. User Experience

- **No UI** in MVP; users interact via REST or existing CLI.
- Clear success & error JSON responses following API standards.
- Documentation examples in Swagger & docs site.
- Copy-button in docs for header usage.
- Accessibility not applicable (no UI).

---

## 6. High-Level Technical Constraints

- **Database** – Postgres; new `users` & `api_keys` tables with hashed key storage and created/last_used timestamps.
- **Key format** – `cpzy_<22-char Base62 hash>`; stored hashed with bcrypt.
- **Rate limiting** – Re-use existing middleware; apply limits per key.
- **Security** – TLS enforced; keys never logged; threat model in Appendix.
- **Performance** – ≤ 5 ms P99 overhead; cache keys in memory (30 s TTL) to minimise DB hits.
- **Compliance** – No immediate GDPR/SOC2, but design allows opt-in logging redaction.

---

## 7. Non-Goals

- OAuth, SSO or social login.
- Multi-tenant key scoping.
- UI dashboard for key management.
- Fine-grained RBAC beyond `admin`/`user`.

---

## 8. Phased Roll-out

| Phase | Scope             | Notes                                                   |
| ----- | ----------------- | ------------------------------------------------------- |
| P0    | Internal dog-food | Admins only, generate keys, validate middleware latency |
| P1    | Private beta      | Selected early users; enable full key CRUD              |
| P2    | Public preview    | Default private workflows; public ones opt-out          |

---

## 9. Success Metrics

- **Auth Coverage** – % requests with valid key (target ≥ 95 %).
- **Middleware Latency** – Added P99 latency (target ≤ 5 ms).
- **Security Events** – Unauthorized requests blocked (≥ 99 %).
- **Key Management Adoption** – % users who generate ≥ 1 key within first week (target 80 %).
- **Incident Count** – Auth-related P1 incidents ≤ 1 in first 30 days.

---

## 10. Risks & Mitigations

| Risk                       | Likelihood | Impact | Mitigation                                        |
| -------------------------- | ---------- | ------ | ------------------------------------------------- |
| Legacy coupling discovered | Low        | Medium | Code audit; document follow-up tasks              |
| Key leakage                | Medium     | High   | Hash at rest, never log raw keys, easy revocation |
| Cache sync issues          | Low        | Medium | Short TTL; fail-open to DB lookup                 |
| Abuse via stolen keys      | Low        | High   | Rate-limit per key; monitor unusual patterns      |

---

## 11. Open Questions

1. Key expiration policy – rotate annually or user-managed?
2. CLI command namespace – integrate under existing `compozy` root or new sub-command group?
3. Should admin be able to downgrade themselves (lockout risk)?

---

## 12. Appendix

### A. Basic Threat Model (MVP)

- **Assets** – Workflow execution, user data.
- **Actors** – External attacker, malicious insider.
- **Entry Points** – API endpoints accepting keys.
- **Controls** – HTTPS, bcrypt-hashed keys, rate-limit, revocation.
- **Gaps** – No MFA, no anomaly detection in MVP.

### B. Abuse & Fraud Considerations

- Credential-stuffing protection via per-key rate limit.
- Monitor failed-auth metrics; alert on spikes.

### C. Technical Spike Plan

| Goal                                  | Success Indicator                   |
| ------------------------------------- | ----------------------------------- |
| Prototype Key generation & validation | Working PoC within 2 days           |
| Evaluate Auth0/Cognito integration    | Comparison doc with pros/cons       |
| Decision Matrix Criteria              | Cost, latency, lock-in, roadmap fit |

### D. Edge-Case Stories

- Revoking last active key must not terminate current session immediately to avoid locking out automation – provide grace period.
- Admin deleting themselves forbidden unless another admin exists.

---

_End of Document_
