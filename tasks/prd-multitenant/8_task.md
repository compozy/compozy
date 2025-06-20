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

# Task 8.0: Multi-Tenant Data Access Layer

## Overview

Implement organization-scoped data access patterns and update existing repositories for multi-tenant support. This task ensures complete data isolation between organizations at the repository level.

## Subtasks

- [ ] 8.1 Add org_id filtering to all existing queries (workflows, tasks, schedules)
- [ ] 8.2 Create OrganizationContext helper for automatic query filtering
- [ ] 8.3 Update WorkflowRepository, TaskRepository, ScheduleRepository with organization scope
- [ ] 8.4 Implement middleware integration for automatic organization context injection
- [ ] 8.5 Add prevention of cross-organization data access at repository level
- [ ] 8.6 Update all CRUD operations to include org_id in queries
- [ ] 8.7 Optimize queries with proper index usage for performance
- [ ] 8.8 Implement error handling that prevents cross-organization information leakage

## Implementation Details

Update existing repositories in engine/infra/store:

1. **Add org_id filtering** to all existing queries (workflows, tasks, schedules)
2. **Create OrganizationContext helper** for automatic query filtering
3. **Update WorkflowRepository, TaskRepository, ScheduleRepository** with organization scope
4. **Implement middleware integration** for automatic organization context injection
5. **Prevent cross-organization data access** at repository level
6. **Update all CRUD operations** to include org_id
7. **Optimize queries** with proper index usage
8. **Error handling** that doesn't leak cross-organization information

Ensure all queries use composite indexes (org_id, created_at) for performance.

## Success Criteria

- All existing queries properly filtered by org_id
- OrganizationContext helper automatically applies filtering
- Repository updates maintain backward compatibility where possible
- Middleware integration seamlessly injects organization context
- Cross-organization data access completely prevented
- All CRUD operations properly scoped to organization
- Query performance optimized through proper index usage
- Error handling prevents information leakage between organizations

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
