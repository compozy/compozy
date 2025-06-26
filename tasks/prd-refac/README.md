# Normalizer Package Refactoring

This directory contains the Product Requirements Document (PRD) and Technical Specification for refactoring the Compozy normalizer package from a monolithic architecture to a modular, task-type-specific architecture.

## Overview

The normalizer package is responsible for:

- Configuration normalization across different task types
- Template processing and variable substitution
- Environment merging (workflow → task → component)
- Context building for template evaluation
- Output transformation

## Documents

- **[\_prd.md](_prd.md)** - Product Requirements Document outlining the business goals, requirements, and success criteria
- **[\_techspec.md](_techspec.md)** - Technical Specification detailing the architecture, implementation plan, and migration strategy

## Key Changes

### Current State

- Monolithic `pkg/normalizer/` package handling all task types
- Type-specific logic embedded in main normalizer with if/else branching
- Tight coupling between normalizer and all task configurations

### Target State

- Modular architecture under `engine/task2/`
- Each task type has its own normalizer package
- Factory pattern for creating appropriate normalizers
- Shared components properly extracted and reused
- Full compliance with SOLID principles

## Architecture Overview

```
engine/task2/
├── normalizer.go           # Core interfaces
├── factory.go              # NormalizerFactory
├── shared/                 # Shared components
├── core/                   # Core services
│   ├── config_normalizer.go    # Main orchestration
│   ├── output_transformer.go   # Output normalization
│   ├── env_merger.go           # Environment merging
│   ├── agent_normalizer.go     # Agent component normalization
│   ├── tool_normalizer.go      # Tool component normalization
│   ├── success_transition.go   # Success transition normalization
│   └── error_transition.go     # Error transition normalization
├── basic/                  # Basic task normalizer
├── parallel/               # Parallel task normalizer
├── collection/             # Collection task normalizer
├── router/                 # Router task normalizer
├── wait/                   # Wait task normalizer
├── aggregate/              # Aggregate task normalizer
├── composite/              # Composite task normalizer
└── signal/                 # Signal task normalizer
```

## Benefits

1. **Maintainability**: Each task type is isolated in its own package
2. **Extensibility**: New task types can be added without modifying existing code
3. **Testability**: Each normalizer can be tested in isolation
4. **Clarity**: Clear separation of concerns and consistent patterns

## Getting Started

To work on this refactoring:

1. Review the PRD to understand the business requirements
2. Study the technical specification for implementation details
3. Follow the phased implementation plan outlined in the tech spec
4. Ensure all changes comply with project standards in `.cursor/rules/`

## Project Standards

This refactoring must adhere to:

- [Architecture & Design Principles](../../.cursor/rules/architecture.mdc)
- [Go Coding Standards](../../.cursor/rules/go-coding-standards.mdc)
- [Testing Standards](../../.cursor/rules/testing-standards.mdc)
- [Core Go Patterns](../../.cursor/rules/go-patterns.mdc)
