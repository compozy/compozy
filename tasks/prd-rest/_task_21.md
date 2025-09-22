## markdown

## status: pending

<task_context>
<domain>engine/infra/docs</domain>
<type>documentation</type>
<scope>configuration</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 21.0: Swagger Drift Gate & Documentation Polish

## Overview

Add a CI gate to fail on Swagger drift and polish documentation for typed endpoints.

<import>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</import>

<requirements>
- Add CI step to run Swagger generation (project target) and `git diff --exit-code`.
- Ensure all endpoints document headers: `ETag`, `Location`, `Link`, `RateLimit-*`.
- Add minimal response examples for core DTOs.
</requirements>

## Subtasks

- [ ] 21.1 Add CI step to run swagger generation and fail on diff.
- [ ] 21.2 Audit annotations for required headers across endpoints.
- [ ] 21.3 Add examples for DTOs where valuable.

## Implementation Details

Integrate with existing Make targets (swagger-gen) and ensure the pipeline validates artifacts.

### Relevant Files

- `Makefile`
- `docs/docs.go`, `docs/swagger.json`, `docs/swagger.yaml`

### Dependent Files

- `engine/*/router/*.go`

## Success Criteria

- CI fails on uncommitted Swagger changes.
- Headers and examples present where applicable.
