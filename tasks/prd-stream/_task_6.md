## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>docs</domain>
<type>documentation</type>
<scope>core_feature</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 6.0: Swagger + Docs Updates

## Overview

Add swagger annotations for three /stream endpoints with text/event-stream content type. Update API pages, overview, schema event catalog, and CLI notes per \_docs.md.

<critical>
- **MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc
- swagger.json, swagger.yaml and docs.go are auto generated through the make swagger-gen so you don't need to modify them directly, but add a proper comments to generate this
</critical>

<requirements>
- Swagger: add three GET operations, headers (Last-Event-ID), query (poll_ms, events), content type.
- Docs: update pages listed in _docs.md; include EventSource examples.
- Build docs locally to ensure no missing routes.
</requirements>

## Subtasks

- [ ] 6.1 Swagger annotations for handlers and regen swagger.json
- [ ] 6.2 Update API pages and overview
- [ ] 6.3 Add schema page for event catalog

## Implementation Details

Follow Docs Plan and Tech Spec sections on endpoints and event types.

### Relevant Files

- docs/content/docs/api/\*
- docs/content/docs/schema/execution-stream-events.mdx
- docs/swagger.yaml (generated)

## Deliverables

- Updated docs and swagger; build succeeds

## Tests

- Docs acceptance per \_docs.md criteria

## Success Criteria

- Endpoints documented and discoverable; swagger renders
