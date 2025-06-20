---
status: pending
---

<task_context>
<domain>engine/infra/store</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 3.0: Repository Layer Implementation

## Overview

Implement repository interfaces and PostgreSQL implementations for Organization, User, and APIKey entities with organization-scoped queries. This layer provides data access with built-in multi-tenant isolation.

## Subtasks

- [ ] 3.1 Create OrgRepository interface with CRUD operations, FindByName, and UpdateStatus methods
- [ ] 3.2 Create UserRepository interface with organization-scoped queries, FindByEmail, and UpdateRole methods
- [ ] 3.3 Create APIKeyRepository interface with FindByPrefix, ValidateKey, and organization-scoped listing
- [ ] 3.4 Implement PostgreSQL implementations using pgx with prepared statements for all repositories
- [ ] 3.5 Ensure all queries include org_id filtering where applicable for data isolation
- [ ] 3.6 Add transaction support using store.TransactionManager for atomic operations
- [ ] 3.7 Implement proper error handling with domain-specific errors following project patterns
- [ ] 3.8 Configure connection pooling with organization-aware patterns for optimal performance

## Implementation Details

Create repository layer in engine/infra/store:

1. **OrgRepository interface** with CRUD operations, FindByName, UpdateStatus methods
2. **UserRepository interface** with organization-scoped queries, FindByEmail, UpdateRole methods
3. **APIKeyRepository interface** with FindByPrefix, ValidateKey, organization-scoped listing
4. **PostgreSQL implementations** using pgx with prepared statements
5. **All queries must include org_id filtering** where applicable
6. **Transaction support** using store.TransactionManager
7. **Proper error handling** with domain-specific errors
8. **Connection pooling** with organization-aware patterns

Implement repository pattern with interfaces for testability.

## Success Criteria

- Repository interfaces properly defined with organization-scoped operations
- PostgreSQL implementations use prepared statements for optimal performance
- All queries include org_id filtering for complete data isolation
- Transaction support ensures atomic multi-step operations
- Error handling follows project standards with appropriate error types
- Connection pooling configured for multi-tenant workloads
- Repository pattern supports easy testing with mock implementations

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
