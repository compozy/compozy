---
status: completed
---

<task_context>
<domain>engine/infra/auth</domain>
<type>implementation</type>
<scope>middleware</scope>
<complexity>high</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 7.0: Authentication and Authorization Middleware

## Overview

Implement Gin middleware for API key authentication, organization context injection, and rate limiting. This middleware provides the security layer for all API requests with proper multi-tenant isolation.

<critical>
**MANDATORY REQUIREMENTS:**
- **ALWAYS** verify against PRD and tech specs - NEVER make assumptions
- **NEVER** use workarounds, especially in tests - implement proper solutions
- **MUST** follow all established project standards:
    - Architecture patterns: `.cursor/rules/architecture.mdc`
    - Go coding standards: `.cursor/rules/go-coding-standards.mdc`
    - Testing requirements: `.cursor/rules/testing-standards.mdc`
    - API standards: `.cursor/rules/api-standards.mdc`
    - Security & quality: `.cursor/rules/quality-security.mdc`
- **MUST** run `make lint` and `make test` before completing ANY subtask
- **MUST** follow `.cursor/rules/task-completion.mdc` workflow for parent tasks
**Enforcement:** Violating these standards results in immediate task rejection.
</critical>

## Subtasks

- [x] 7.1 Create AuthMiddleware for Bearer token extraction and API key validation
- [x] 7.2 Implement OrgContextMiddleware for organization context injection into request context
- [x] 7.3 Add RateLimitMiddleware using in-memory rate limiter with golang.org/x/time/rate
- [x] 7.4 Implement proper error responses following project standard format
- [x] 7.5 Add context propagation with apiKey, user, organization, and userRole
- [x] 7.6 Configure rate limiting per API key with configurable limits (100 req/sec, burst 20)
- [x] 7.7 Add security headers and comprehensive audit logging
- [x] 7.8 Implement graceful error handling with appropriate HTTP status codes

## Implementation Details

Create middleware in engine/infra/auth:

1. **AuthMiddleware** for Bearer token extraction and API key validation
2. **OrgContextMiddleware** for organization context injection into request context
3. **RateLimitMiddleware** using in-memory rate limiter with golang.org/x/time/rate
4. **Proper error responses** following project standard format
5. **Context propagation** with apiKey, user, organization, and userRole
6. **Rate limiting** per API key with configurable limits (100 req/sec, burst 20)
7. **Security headers** and audit logging
8. **Graceful error handling** with appropriate HTTP status codes

Use sync.RWMutex for thread-safe rate limiter map with cleanup goroutine.

### Relevant Files

- `engine/auth/middleware.go` - Authentication and authorization middleware
- `engine/auth/ratelimit/service.go` - Rate limiting implementation with cleanup
- `engine/auth/context.go` - Organization context injection and propagation
- `engine/auth/security_headers.go` - Security headers configuration

## Success Criteria

- API key authentication properly validates Bearer tokens
- Organization context successfully injected into all authenticated requests
- Rate limiting enforced per API key with configurable limits
- Error responses follow project standard format
- Context propagation includes all required authentication data
- Security headers properly set for all responses
- Audit logging captures all authentication events
- Thread-safe operation under concurrent request load
