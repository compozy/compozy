# PRD Template

Use this template to structure every Product Requirements Document. The PRD is consumed by LLM agents downstream (`cy-create-techspec`, `cy-create-tasks`, `cy-execute-task`, review rounds): it exists to give them business rules, domain behavior, and product intent.

Fill each section based on brainstorming outcomes. Leave placeholder guidance in sections where information is insufficient and note them in Open Questions.

## Overview

High-level overview of the feature or product. Describe:
- What problem it solves
- Who it is for
- Why it is valuable

## Goals

Product outcomes stated as observable behavior, not metrics:
- What users can do after this ships that they could not do before
- What the system guarantees or enforces once the feature exists
- What becomes unnecessary, automatic, or impossible for users

## User Stories

Index into `_user_stories.md`, the canonical story catalog — do not restate stories here:
- One line per feature area: the `US-NNN` range it covers and its theme
- Link the catalog: [Full user stories](_user_stories.md)

## Core Features

Main features of the product:
- Feature name: what it does, why it is important, high-level behavior
- Functional requirements for each feature
- Interaction between features

## Business Rules

Domain rules the implementation must enforce, stated precisely:
- Invariants that must always hold (e.g., "a run belongs to exactly one workspace")
- Validation rules and their user-facing outcomes
- Permission and visibility rules per persona
- Lifecycle and state-transition rules (which states exist, what may move where, and when)
- Calculations, limits, and defaults with their exact values

## User Experience

User journey from first contact to regular use:
- Key personas and their goals
- Primary user flows step by step
- UI/UX considerations and accessibility requirements
- Onboarding and discoverability

## High-Level Technical Constraints

Required boundaries that shape the product without prescribing implementation:
- Required integrations with existing systems
- Compliance mandates or regulatory requirements
- Performance targets from a user perspective
- Data privacy and security requirements

Implementation choices — databases, frameworks, API designs, architecture patterns — belong to the TechSpec.

## Non-Goals (Out of Scope)

Capabilities the user decided this feature will not include:
- Adjacent problems that will not be addressed, and why
- Boundaries of this effort

Exclusions record user decisions, never size management: a wanted capability stays in scope no matter how large the document grows.

## Architecture Decision Records

ADRs documenting key decisions made during brainstorming:
- [ADR-NNN: Title](adrs/adr-NNN.md) — One-line summary of the decision

## Open Questions

Remaining items that need clarification:
- Unclear requirements
- Edge cases requiring stakeholder input
- Dependencies on decisions not yet made
