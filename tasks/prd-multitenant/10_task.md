---
status: pending
---

<task_context>
<domain>engine/api</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 10.0: REST API Endpoints

## Overview

Implement comprehensive REST API endpoints for organization, user, and API key management with proper validation and error handling. This provides the external interface for multi-tenant operations.

## Subtasks

- [ ] 10.1 Create organization endpoints: POST/GET/PUT/DELETE /api/v0/organizations
- [ ] 10.2 Create user endpoints: POST/GET/PUT/DELETE /api/v0/organizations/{org_id}/users
- [ ] 10.3 Create API key endpoints: POST/GET/PUT/DELETE /api/v0/users/{user_id}/api-keys
- [ ] 10.4 Implement proper request validation using github.com/go-playground/validator/v10
- [ ] 10.5 Ensure consistent response format: {"data": {...}, "message": "..."}
- [ ] 10.6 Add role-based access control for each endpoint
- [ ] 10.7 Implement pagination support for list endpoints
- [ ] 10.8 Add proper HTTP status codes and comprehensive error responses

## Implementation Details

Create API handlers in engine/api:

1. **Organization endpoints**: POST/GET/PUT/DELETE /api/v0/organizations
2. **User endpoints**: POST/GET/PUT/DELETE /api/v0/organizations/{org_id}/users
3. **API key endpoints**: POST/GET/PUT/DELETE /api/v0/users/{user_id}/api-keys
4. **Proper request validation** using github.com/go-playground/validator/v10
5. **Consistent response format**: {"data": {...}, "message": "..."}
6. **Role-based access control** for each endpoint
7. **Pagination support** for list endpoints
8. **Proper HTTP status codes** and error responses

Implement Gin router with middleware chain: auth -> rate limit -> org context -> handler.

## Success Criteria

- All CRUD endpoints properly implemented for organizations, users, and API keys
- Request validation prevents invalid data submission
- Response format consistent across all endpoints
- Role-based access control properly enforced
- Pagination implemented for efficient data retrieval
- HTTP status codes accurately reflect operation results
- Error responses provide clear, actionable feedback
- Middleware chain ensures proper authentication and context

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
