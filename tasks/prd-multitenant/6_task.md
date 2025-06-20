---
status: pending
---

<task_context>
<domain>engine/infra/auth</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>medium</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 6.0: User Management Service

## Overview

Implement user lifecycle management with role-based permissions and organization-scoped operations. This service manages user accounts within the multi-tenant system with proper access control.

## Subtasks

- [ ] 6.1 Implement user CRUD operations with organization context validation
- [ ] 6.2 Add role management (admin/manager/customer) with permission validation
- [ ] 6.3 Implement email uniqueness validation within organization scope
- [ ] 6.4 Add user status management (active/suspended) with proper state transitions
- [ ] 6.5 Implement role-based permission checking for operations
- [ ] 6.6 Add user-organization association validation to prevent cross-tenant access
- [ ] 6.7 Implement bulk user operations for organization management
- [ ] 6.8 Add user activity tracking and audit logging for security compliance

## Implementation Details

Create UserService in engine/infra/auth:

1. **User CRUD operations** with organization context
2. **Role management** (admin/manager/customer) with permission validation
3. **Email uniqueness validation** within organization scope
4. **User status management** (active/suspended)
5. **Role-based permission checking** for operations
6. **User-organization association** validation
7. **Bulk user operations** for organization management
8. **User activity tracking** and audit logging

Implement permission matrix: admin (global), manager (org-wide), customer (read/execute only).

## Success Criteria

- User operations properly scoped to organization boundaries
- Role-based permissions correctly enforced for all operations
- Email uniqueness maintained within each organization
- User status transitions follow business rules
- Permission validation prevents unauthorized access
- User-organization associations properly validated
- Bulk operations enable efficient organization management
- Activity tracking provides comprehensive audit trail

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
