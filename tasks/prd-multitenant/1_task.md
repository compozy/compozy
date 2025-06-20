---
status: completed
---

<task_context>
<domain>engine/infra/store</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>database</dependencies>
</task_context>

# Task 1.0: Database Schema Design and Migration

## Overview

Design PostgreSQL database schema for multi-tenant system with Organizations, Users, and APIKeys tables. This foundation enables secure, isolated data storage with proper relationships and constraints.

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

- [x] 1.1 Create Organizations table with ID, Name, TemporalNamespace, Status (active/suspended), CreatedAt, UpdatedAt
- [x] 1.2 Create Users table with ID, OrgID, Email, Role (admin/manager/customer), Status, CreatedAt, UpdatedAt
- [x] 1.3 Create APIKeys table with ID, UserID, OrgID, Hash, Name, ExpiresAt, RateLimitPerHour, CreatedAt, UpdatedAt, Status
- [x] 1.4 Create migration with proper foreign key constraints and cascade behavior
- [x] 1.5 Add composite indexes for performance: (org_id, created_at), (user_id, org_id), etc.
- [x] 1.6 Add unique constraints: email per organization, organization name globally, API key prefix
- [x] 1.7 Design role-based access control schema with permission mapping
- [x] 1.8 Update existing tables (workflows, tasks, schedules) to include org_id column

## Implementation Details

Design PostgreSQL schema in migrations:

1. **Organizations table** with ID, Name, TemporalNamespace, Status (active/suspended), timestamps
2. **Users table** with ID, OrgID, Email, Role (admin/manager/customer), Status, timestamps
3. **APIKeys table** with ID, UserID, OrgID, Hash, Name, ExpiresAt, RateLimitPerHour, timestamps, Status
4. **Foreign key constraints** with proper cascade behavior
5. **Composite indexes** for performance: (org_id, created_at), (user_id, org_id)
6. **Unique constraints**: email per organization, organization name globally
7. **Role-based access control** schema with permission mapping
8. **Update existing tables** (workflows, tasks, schedules) with org_id column

Use UUIDs for all primary keys. Implement soft deletes with deleted_at timestamp.

### Relevant Files

- `engine/infra/store/migrations/[timestamp]_add_multitenant_schema.sql` - Database migration for multi-tenant tables
- `engine/infra/store/migrations/[timestamp]_update_existing_tables_for_multitenancy.sql` - Migration to add org_id to existing tables
- `engine/infra/store/schema.sql` - Updated schema with all table definitions

## Success Criteria

- Database schema supports complete multi-tenant isolation with proper relationships
- Foreign key constraints ensure referential integrity
- Composite indexes optimize organization-scoped queries
- Unique constraints prevent data conflicts within tenant boundaries
- Role-based access control schema enables fine-grained permissions
- Existing tables properly updated for multi-tenant support without data loss
- Migration scripts are reversible and tested
- Schema design supports future scalability requirements
