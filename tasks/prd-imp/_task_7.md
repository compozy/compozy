---
status: pending
parallelizable: true
blocked_by: ["3.0", "4.0", "5.0", "6.0"]
---

<task_context>
<domain>engine/infra/server</domain>
<type>implementation</type>
<scope>cleanup</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
<unblocks>"8.0"</unblocks>
</task_context>

# Task 7.0: Remove admin routes and references

## Overview

Remove legacy admin import/export endpoints and references in code and docs per “No Backwards Compatibility” rule.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Delete `engine/infra/server/reg_admin_export.go` and `reg_admin_import.go`
- Remove admin registration lines from `reg_admin.go` and adjust tests
- Ensure Swagger no longer references `/admin/(import|export)-yaml`
</requirements>

## Subtasks

- [ ] 7.1 Delete files and unregister routes
- [ ] 7.2 Update/clean tests and docs
- [ ] 7.3 `rg -n "/admin/(import|export)-yaml"` returns no matches in repo

## Success Criteria

- No `/admin/(import|export)-yaml` in codebase or Swagger
