---
status: pending
---

<task_context>
<domain>engine/infra/auth</domain>
<type>implementation</type>
<scope>core_feature</scope>
<complexity>high</complexity>
<dependencies>external_apis</dependencies>
</task_context>

# Task 4.0: API Key Security Service

## Overview

Implement secure API key generation, hashing, and validation service using Argon2 with constant-time comparison. This service provides the cryptographic foundation for secure multi-tenant authentication.

## Subtasks

- [ ] 4.1 Implement secure 32-character random API key generation with 'cmpz\_' prefix
- [ ] 4.2 Implement Argon2 hashing with salt generation for secure API key storage
- [ ] 4.3 Implement constant-time comparison using subtle.ConstantTimeCompare to prevent timing attacks
- [ ] 4.4 Create key validation system with organization and user context retrieval
- [ ] 4.5 Add support for key expiration checking and validation
- [ ] 4.6 Implement rate limiting configuration per API key
- [ ] 4.7 Add comprehensive audit logging for key generation, validation attempts, and security events
- [ ] 4.8 Implement key prefix extraction for efficient database lookups

## Implementation Details

Create APIKeyService in engine/infra/auth:

1. **Generate 32-character random API keys** with 'cmpz\_' prefix
2. **Implement Argon2 hashing** with salt generation for secure storage
3. **Constant-time comparison** using subtle.ConstantTimeCompare to prevent timing attacks
4. **Key validation** with organization and user context retrieval
5. **Support for key expiration** checking
6. **Rate limiting configuration** per API key
7. **Audit logging** for key generation, validation attempts, and security events
8. **Key prefix extraction** for efficient database lookups

Use golang.org/x/crypto/argon2 with secure parameters: time=1, memory=64\*1024, threads=4, keyLen=32.

## Success Criteria

- API keys generated with cryptographically secure randomness
- Argon2 hashing implemented with secure parameters and salt generation
- Timing attack resistance verified through constant-time comparison
- Key validation retrieves complete organization and user context
- Expiration checking prevents use of expired keys
- Rate limiting configuration properly associated with API keys
- Comprehensive audit logging captures all security-relevant events
- Performance optimized through efficient prefix-based database lookups

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
