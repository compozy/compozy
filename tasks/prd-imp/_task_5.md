---
status: pending
parallelizable: true
blocked_by: ["3.0", "4.0"]
---

<task_context>
<domain>docs/swagger</domain>
<type>documentation</type>
<scope>api</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"8.0"</unblocks>
</task_context>

# Task 5.0: Swagger updates and regeneration

## Overview

Update Swagger annotations to include new per‑resource import/export endpoints and remove admin import/export references. Regenerate `docs/*`.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add `@Router /{resource}/export [post]` and `/import [post]` in each new handler
- Ensure `@Tags`, `@Security ApiKeyAuth` (if applicable), and response envelopes are correct
- Remove admin import/export annotations
- Regenerate `docs/` artifacts and verify no `/admin/*` paths remain
</requirements>

## Subtasks

- [ ] 5.1 Update annotations in all resource routers
- [ ] 5.2 Regenerate docs (e.g., `swag init` per repo practice) and verify
- [ ] 5.3 `rg -n "/admin/(import|export)-yaml" docs/ || true` returns no matches

## Success Criteria

- Swagger contains only per‑resource import/export endpoints
