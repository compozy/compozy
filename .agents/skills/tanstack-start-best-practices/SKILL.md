---
name: tanstack-start-best-practices
description: TanStack Start best practices for full-stack React applications. Server functions, middleware, SSR, authentication, and deployment patterns. Activate when building full-stack apps with TanStack Start.
---

# TanStack Start Best Practices

Comprehensive guidelines for implementing TanStack Start patterns in full-stack React applications. These rules cover server functions, middleware, SSR, authentication, and deployment.

## When to Apply

- Creating server functions for data mutations
- Setting up middleware for auth/logging
- Configuring SSR and hydration
- Implementing authentication flows
- Handling errors across client/server boundary
- Organizing full-stack code
- Deploying to various platforms

## Rule Categories by Priority

| Priority | Category          | Rules   | Impact                      |
| -------- | ----------------- | ------- | --------------------------- |
| CRITICAL | Server Functions  | 5 rules | Core data mutation patterns |
| CRITICAL | Security          | 4 rules | Prevents vulnerabilities    |
| HIGH     | Middleware        | 4 rules | Request/response handling   |
| HIGH     | Authentication    | 4 rules | Secure user sessions        |
| MEDIUM   | API Routes        | 1 rule  | External endpoint patterns  |
| MEDIUM   | SSR               | 6 rules | Server rendering patterns   |
| MEDIUM   | Error Handling    | 3 rules | Graceful failure handling   |
| MEDIUM   | Environment       | 1 rule  | Configuration management    |
| LOW      | File Organization | 3 rules | Maintainable code structure |
| LOW      | Deployment        | 2 rules | Production readiness        |

## Quick Reference

### Server Functions (Prefix: `sf-`)

- `sf-create-server-fn` — Use createServerFn for server-side logic
- `sf-input-validation` — Always validate server function inputs
- `sf-method-selection` — Choose appropriate HTTP method
- `sf-error-handling` — Handle errors in server functions
- `sf-response-headers` — Customize response headers when needed

### Security (Prefix: `sec-`)

- `sec-validate-inputs` — Validate all user inputs with schemas
- `sec-auth-middleware` — Protect routes with auth middleware
- `sec-sensitive-data` — Keep secrets server-side only
- `sec-csrf-protection` — Implement CSRF protection for mutations

### Middleware (Prefix: `mw-`)

- `mw-request-middleware` — Use request middleware for cross-cutting concerns
- `mw-function-middleware` — Use function middleware for server functions
- `mw-context-flow` — Properly pass context through middleware
- `mw-composability` — Compose middleware effectively

### Authentication (Prefix: `auth-`)

- `auth-session-management` — Implement secure session handling
- `auth-route-protection` — Protect routes with beforeLoad
- `auth-server-functions` — Verify auth in server functions
- `auth-cookie-security` — Configure secure cookie settings

### API Routes (Prefix: `api-`)

- `api-routes` — Create API routes for external consumers

### SSR (Prefix: `ssr-`)

- `ssr-data-loading` — Load data appropriately for SSR
- `ssr-hydration-safety` — Prevent hydration mismatches
- `ssr-streaming` — Implement streaming SSR for faster TTFB
- `ssr-selective` — Apply selective SSR when beneficial
- `ssr-prerender` — Configure static prerendering and ISR

### Environment (Prefix: `env-`)

- `env-functions` — Use environment functions for configuration

### Error Handling (Prefix: `err-`)

- `err-server-errors` — Handle server function errors
- `err-redirects` — Use redirects appropriately
- `err-not-found` — Handle not-found scenarios

### File Organization (Prefix: `file-`)

- `file-separation` — Separate server and client code
- `file-functions-file` — Use .functions.ts pattern
- `file-shared-validation` — Share validation schemas

### Deployment (Prefix: `deploy-`)

- `deploy-env-config` — Configure environment variables
- `deploy-adapters` — Choose appropriate deployment adapter

## How to Use

Each rule file in the `rules/` directory contains:

1. **Explanation** — Why this pattern matters
2. **Bad Example** — Anti-pattern to avoid
3. **Good Example** — Recommended implementation
4. **Context** — When to apply or skip this rule

## Full Reference

See individual rule files in `rules/` directory for detailed guidance and code examples.
