---
status: pending
---

<task_context>
<domain>engine/infra/auth</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>temporal</dependencies>
</task_context>

# Task 5.0: Organization Management Service

## Overview

Implement organization lifecycle management service with Temporal namespace provisioning and transaction handling. This service orchestrates the complete organization setup process with rollback capabilities.

## Subtasks

- [ ] 5.1 Implement CreateOrganization with atomic database + Temporal namespace creation
- [ ] 5.2 Add Temporal namespace provisioning with retry logic and exponential backoff
- [ ] 5.3 Implement organization status management (provisioning -> active -> suspended)
- [ ] 5.4 Add transaction handling for multi-step operations with proper rollback
- [ ] 5.5 Implement namespace naming convention: 'org-{org-slug}-{short-uuid}' format
- [ ] 5.6 Add comprehensive error handling with proper rollback on Temporal failures
- [ ] 5.7 Implement organization validation and uniqueness checking
- [ ] 5.8 Integrate with Temporal client for namespace management operations

## Implementation Details

Create OrganizationService in engine/infra/auth:

1. **CreateOrganization** with atomic database + Temporal namespace creation
2. **Temporal namespace provisioning** with retry logic and exponential backoff
3. **Organization status management** (provisioning -> active -> suspended)
4. **Transaction handling** for multi-step operations
5. **Namespace naming**: 'org-{org-slug}-{short-uuid}' format
6. **Error handling** with proper rollback on Temporal failures
7. **Organization validation** and uniqueness checking
8. **Integration** with Temporal client for namespace management

Implement retry.Do with 3 attempts, exponential backoff starting at 500ms, max delay 5s.

## Success Criteria

- Organization creation process is atomic with proper rollback handling
- Temporal namespace provisioning includes retry logic for resilience
- Organization status accurately reflects provisioning state
- Transaction failures properly roll back all changes
- Namespace naming follows established convention and avoids collisions
- Error handling provides clear feedback for different failure scenarios
- Organization validation prevents duplicate or invalid entries
- Temporal client integration supports all required namespace operations

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
