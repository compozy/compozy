---
status: pending
---

<task_context>
<domain>engine/core</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 2.0: Core Domain Entities and Types

## Overview

Implement core domain entities for Organization, User, and APIKey with proper validation and business logic. These entities form the foundation of the multi-tenant system with role-based access control.

## Subtasks

- [ ] 2.1 Create Organization struct with ID, Name, TemporalNamespace, Status (active/suspended), and timestamps
- [ ] 2.2 Create User struct with ID, OrgID, Email, Role (admin/manager/customer), Status, and timestamps
- [ ] 2.3 Create APIKey struct with ID, UserID, OrgID, Hash, Name, ExpiresAt, RateLimitPerHour, timestamps, and Status
- [ ] 2.4 Implement Role enum with proper permission mapping for admin/manager/customer roles
- [ ] 2.5 Implement OrganizationStatus enum with state transitions (provisioning -> active -> suspended)
- [ ] 2.6 Add validation methods for email format and organization name uniqueness
- [ ] 2.7 Implement business logic for namespace generation using 'compozy-{org-slug}' format
- [ ] 2.8 Ensure ID generation uses core.NewID() for consistency across all entities
- [ ] 2.9 Implement proper JSON marshaling/unmarshaling with struct tags

## Implementation Details

Create domain entities in engine/core:

1. **Organization struct** with ID, Name, TemporalNamespace, Status (active/suspended), timestamps
2. **User struct** with ID, OrgID, Email, Role (admin/manager/customer), Status, timestamps
3. **APIKey struct** with ID, UserID, OrgID, Hash, Name, ExpiresAt, RateLimitPerHour, timestamps, Status
4. **Role enum** with proper permission mapping
5. **OrganizationStatus enum** with state transitions
6. **Validation methods** for email format, organization name uniqueness
7. **Business logic** for namespace generation: 'compozy-{org-slug}'
8. **ID generation** using core.NewID() for consistency

Implement proper JSON marshaling/unmarshaling with struct tags.

## Success Criteria

- All domain entities properly defined with required fields and validation
- Role-based permissions clearly mapped and enforceable
- Organization status transitions follow business rules
- Namespace generation produces valid, unique identifiers
- JSON serialization/deserialization works correctly for API usage
- Entity validation prevents invalid data entry
- Business logic enforces multi-tenant isolation requirements

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
