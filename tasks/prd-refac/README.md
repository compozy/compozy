# Engine Refactoring Documentation

This directory contains Product Requirements Documents (PRDs) and Technical Specifications for refactoring critical components of the Compozy engine from monolithic architectures to modular, domain-driven designs.

## Overview

This directory documents the ongoing refactoring efforts to modernize the Compozy engine architecture:

### Normalizer Package

Responsible for:

- Configuration normalization across different task types
- Template processing and variable substitution
- Environment merging (workflow → task → component)
- Context building for template evaluation
- Output transformation

### Task Orchestration Services

Responsible for:

- Task configuration management and storage
- Parent-child task relationship management
- Inter-task signal dispatching
- Wait task lifecycle management
- Task execution response handling

## Refactoring Projects

### 1. Normalizer Package Refactoring

**Documents:**

- **[\_prd.md](_prd.md)** - Product Requirements Document for normalizer refactoring
- **[\_techspec.md](_techspec.md)** - Technical Specification for normalizer architecture
- **[integration-plan.md](integration-plan.md)** - Direct integration strategy for task2 package

### 2. Task Services Migration

**Documents:**

- **[task-services-migration-prd.md](task-services-migration-prd.md)** - Product Requirements for service migration
- **[task-services-migration-techspec.md](task-services-migration-techspec.md)** - Technical architecture and implementation
- **[task-services-migration-tasks.md](task-services-migration-tasks.md)** - Detailed task breakdown (20 tasks)
- **[task-services-migration-plan.md](task-services-migration-plan.md)** - Initial migration strategy

**Scope:** Migration of four critical orchestration services:

- ConfigManager - Task configuration and metadata management
- ParentStatusUpdater - Parent task status tracking and updates
- SignalDispatcher - Inter-task signal routing
- WaitTaskManager - Wait task lifecycle orchestration

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

All refactoring efforts must adhere to:

- [Architecture & Design Principles](../../.cursor/rules/architecture.mdc)
- [Go Coding Standards](../../.cursor/rules/go-coding-standards.mdc)
- [Testing Standards](../../.cursor/rules/testing-standards.mdc)
- [Core Go Patterns](../../.cursor/rules/go-patterns.mdc)

## Combined Benefits

Together, these refactoring efforts will:

1. **Transform the engine architecture** from monolithic to modular, domain-driven design
2. **Enable independent development** of task types and orchestration services
3. **Improve system reliability** through better isolation and error handling
4. **Accelerate feature development** with cleaner, more maintainable code
5. **Simplify testing** through well-defined interfaces and boundaries

## Current Status

- **Normalizer Refactoring**: Task2 package complete, ready for integration
- **Task Services Migration**: Planning complete, ready for implementation (10-day timeline)
