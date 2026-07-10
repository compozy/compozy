# E2E Coverage Guide

Read this reference when deciding whether the repository already supports automated end-to-end coverage and when that coverage must be added or updated.

## What Counts as E2E

Treat E2E as regression coverage that exercises a public user or operator interface:

- Browser flows through the actual UI
- HTTP API flows through real entrypoints
- CLI or worker flows triggered through documented commands

Do not treat isolated unit or private helper tests as E2E proof.

## High-Confidence Support Signals

Treat the repository as E2E-capable when at least one runnable command exists and at least one of these signals confirms the harness:

- Explicit commands such as `e2e`, `test:e2e`, `playwright`, `cypress`, or `acceptance`
- Framework configs such as `playwright.config.*`, `cypress.config.*`, or `wdio.conf.*`
- Existing spec directories such as `e2e/`, `tests/e2e/`, `test/e2e/`, or `cypress/e2e/`
- CI workflows that run the same E2E command

If only a weak signal exists, do not overclaim support. Record the ambiguity and confirm manually from repository docs or CI.

## Flow Classification

Classify each changed or regression-critical public flow as one of:

- `existing-e2e`: matching automated coverage already exists and remains valid
- `needs-e2e`: repository supports E2E but the flow lacks adequate coverage
- `manual-only`: coverage intentionally stays manual because automation is the wrong tool
- `blocked`: repository supports E2E but required credentials, data, services, or environment are missing

## Balanced Enforcement Policy

Require new or updated E2E coverage when repository support already exists and any of these are true:

- The flow is P0 or P1
- The flow is release-critical smoke coverage
- A bug fix restores a public regression
- A browser, HTTP, or CLI flow is exercised manually as part of the QA proof and no equivalent automated regression exists

Keep flows `manual-only` when they are primarily exploratory, usability-focused, or visual-design judgments that do not fit stable automation.

## Evidence Rules

When E2E support exists:

- Record the canonical E2E command and any narrower spec command used for the fix
- Record each required flow and its classification
- List spec paths that were added or updated
- Re-run the narrow spec plus the canonical E2E command, or the smallest repository-defined subset that covers the touched critical flows

When support does not exist:

- Do not bootstrap a new framework during QA
- Keep live manual evidence
- Report the gap explicitly as `manual-only` or `blocked`
