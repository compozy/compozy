## markdown

## status: pending # Options: pending, in-progress, completed, excluded

<task_context>
<domain>examples</domain>
<type>documentation</type>
<scope>examples</scope>
<complexity>low</complexity>
<dependencies>http_server</dependencies>
</task_context>

# Task 7.0: Examples — Browser and Node Consumers

## Overview

Create two runnable examples demonstrating consumption of /stream: a browser EventSource client and a Node client.

<critical>**MUST READ BEFORE STARTING** @.cursor/rules/critical-validation.mdc</critical>

<requirements>
- sse-browser: simple HTML + JS; param for exec_id; render events; close on complete.
- sse-node: script using eventsource or fetch streaming; prints events.
- Include README with exact commands and expected outputs.
</requirements>

## Subtasks

- [ ] 7.1 Create examples/stream/sse-browser (index.html, README)
- [ ] 7.2 Create examples/stream/sse-node (app.mjs, README)
- [ ] 7.3 Smoke script to verify stream closes on complete

## Implementation Details

Follow the Examples Plan in \_examples.md.

### Relevant Files

- examples/stream/sse-browser/\*
- examples/stream/sse-node/\*

## Deliverables

- Two examples with READMEs; smoke script

## Tests

- Manual runbooks in README; optional CI smoke using local API

## Success Criteria

- Both examples render events and close on completion without errors
