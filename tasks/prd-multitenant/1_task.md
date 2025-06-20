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

# Task 1.0: Database Schema and Migration Setup

## Overview

Create database schema for multi-tenant entities including organizations, users, and api_keys tables with proper indexes and foreign key relationships. This establishes the foundational data layer for complete multi-tenant isolation.

## Subtasks

- [ ] 1.1 Create organizations table with UUID primary key, name (unique), temporal_namespace (unique), status enum, and timestamps
- [ ] 1.2 Create users table with org_id foreign key, email unique within organization scope, role enum, status enum, and timestamps
- [ ] 1.3 Create api_keys table with user_id and org_id foreign keys, key_hash, name, optional expires_at, rate_limit_per_hour, and timestamps
- [ ] 1.4 Add org_id foreign keys to existing tables (workflows, tasks, schedules, workflow_states, task_states)
- [ ] 1.5 Create composite indexes (org_id, created_at) on all major tables for optimal multi-tenant query performance
- [ ] 1.6 Implement partial unique indexes for email uniqueness within organization scope
- [ ] 1.7 Configure pgx driver with proper connection pooling settings

<CRITICAL>you don't need to use ALTER TABLE, you can modify the schemas directly, because we are in the development phase</CRITICAL>

## Implementation Details

Create PostgreSQL migrations for:

1. **organizations table**: id UUID, name string unique, temporal_namespace string unique, status enum, created_at/updated_at
2. **users table**: id UUID, org_id UUID FK, email string, role enum, status enum, timestamps
3. **api_keys table**: id UUID, user_id UUID FK, org_id UUID FK, key_hash string, name string, expires_at nullable, rate_limit_per_hour int, created_at/last_used_at, status enum
4. **Add org_id foreign keys** to existing tables: workflows, tasks, schedules, workflow_states, task_states
5. **Create composite indexes**: (org_id, created_at) on all major tables
6. **Ensure email uniqueness** within organization scope using partial unique indexes

Use pgx driver with proper connection pooling configuration.

## Success Criteria

- All migration files created and executable
- Foreign key constraints properly enforced
- Composite indexes improve query performance for organization-filtered queries
- Email uniqueness enforced within organization boundaries only
- Database schema supports complete multi-tenant data isolation
- Connection pooling configured for optimal performance

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
