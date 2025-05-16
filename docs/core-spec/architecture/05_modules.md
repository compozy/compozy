# Modules Architecture

## Overview

The Compozy Workflow Engine is designed as a modular system, allowing for easy extension and customization of its components. This document outlines the high-level architecture based on Go Workspaces and the domain-driven design principles for the internal structure of these modules. This approach aims to enhance maintainability, scalability, and clarity across the codebase.

## Go Workspace Structure

```text
compozy-go/
├── go.work                      // Defines the workspace
│
├── engine/                      // 'engine' Go module
│   ├── go.mod
│   ├── agent/
│   │   └── ... (domain packages as previously defined)
│   ├── task/
│   │   └── ...
│   ├── tool/
│   │   └── ...
│   ├── workflow/
│   │   └── ...
│   ├── project/
│   │   └── ...
│   ├── common/
│   │   └── ...
│   ├── orchestrator/
│   │   └── orchestrator.go
│   └── state_manager/
│       └── manager.go
│
├── cli/                         // 'cli' Go module
│   ├── go.mod
│   ├── main.go
│   └── commands/
│       └── ...
│
├── runtimepkg/                  // 'runtimepkg' Go module for Deno runtime
│   ├── go.mod                   // (Potentially minimal or none if pure Deno)
│   └── deno/
│       ├── deno.json
│       ├── ... (rest of former pkg/runtime contents)
│
├── server/                      // 'server' Go module for HTTP API
│   ├── go.mod
│   ├── main.go
│   └── ... (API server files: config.go, router.go, etc.)
│
├── pkg/                         // For shared, reusable Go modules
│   └── corelibs/                // New 'corelibs' Go module
│       ├── go.mod
│       ├── logger/              // Logger package
│       │   ├── logger.go
│       │   └── styles.go
│       ├── nats/                // NATS package
│       │   ├── protocol.go
│       │   ├── server.go
│       │   └── subjects.go
│       └── tplengine/           // Template engine package
│           ├── engine.go
│           └── converter.go
│
├── cmd/
│   └── schemagen/
│       └── generate.go
├── docs/
│   └── ...
├── examples/
│   └── ...
├── schemas/
│   └── ...
├── scripts/
│   └── ...
├── test/
│   └── ...
│
├── .DS_Store
├── .editorconfig
├── .env
├── .gitignore
├── .golangci.yml
├── .prettierignore
├── .prettierrc.toml
├── deno.json
├── deno.lock
├── docker-compose.yml
├── file_contents.txt
└── Makefile
```

The Compozy project will be organized as a Go workspace, defined by a `go.work` file at the root. This workspace will contain several distinct Go modules, each responsible for a specific set of functionalities. This allows for better separation of concerns, independent dependency management where appropriate, and clearer build definitions.

The primary Go modules within the workspace are:

1.  **Engine Module (`engine/`)**: The core of the Compozy Workflow Engine.
2.  **CLI Module (`cli/`)**: The command-line interface for interacting with Compozy.
3.  **Runtime Package Module (`runtimepkg/`)**: Manages the Deno runtime environment for tool and agent execution.
4.  **Core Libraries Module (`pkg/corelibs/`)**: Provides foundational, potentially reusable libraries for logging, NATS communication, and templating.
5.  **Server Module (`server/`)**: (Potentially optional or integrated) Handles the HTTP API for external interactions.
6.  **Build/Development Tools**: Standalone tools like `cmd/schemagen/` for schema generation.

## Core Go Modules Detailed

### 1. Engine Module (`engine/`)

This is the central module containing the primary business logic of the Compozy Workflow Engine. It follows a domain-driven package structure and relies on the `Core Libraries Module` for foundational services.

*   **Module Path (example):** `github.com/compozy/compozy/engine`
*   **Dependencies:** `pkg/corelibs`
*   **Responsibilities:**
    *   Orchestration of workflows, tasks, agents, and tools.
    *   Parsing and validation of all Compozy definition files (`project.yaml`, `workflow.yaml`, etc.).
    *   State management and persistence (interfacing with the `state_manager`).
    *   Execution logic for workflows and tasks.
*   **Key Internal Packages (within `engine/`):**
    *   `agent/`: Domain logic for Agents.
        *   `config/`: Parsing and validation of agent definition files.
        *   `executor/`: Logic for preparing agent execution (interaction with `runtimepkg`).
        *   `state/`: Agent-specific runtime state.
    *   `task/`: Domain logic for Tasks.
        *   `config/`: Parsing and validation of task definition files.
        *   `executor/`: Task execution controller.
        *   `state/`: Task-specific runtime state.
    *   `tool/`: Domain logic for Tools.
        *   `config/`: Parsing and validation of tool definition files.
        *   `state/`: Tool-specific runtime state (if any server-side).
    *   `workflow/`: Domain logic for Workflows.
        *   `config/`: Parsing and validation of workflow definition files.
        *   `executor/`: Workflow execution controller.
        *   `state/`: Workflow-specific runtime state.
        *   `loader/`: Utilities for loading referenced tasks, agents, and tools within a workflow.
    *   `project/`: Logic for parsing and validating the main `compozy.yaml` project file.
        *   `config/`: Parsing and validation.
    *   `common/`: Shared utilities and types used across the engine's domain packages.
        *   `parsing/`: Generic file loaders, schema validation, struct validation.
        *   `state/`: Core state interfaces (e.g., `StateID`), execution state store.
        *   `transition/`: Configuration for task transitions.
        *   `env/`: Environment variable handling.
    *   `orchestrator/`: The central component responsible for initiating and coordinating workflow executions (maps to `system.Orchestrator`).
    *   `state_manager/`: Handles the persistence and retrieval of all state data, potentially using event sourcing (maps to `system.State_Manager`).

### 2. CLI Module (`cli/`)

This module provides the command-line interface for users to interact with Compozy.

*   **Module Path (example):** `github.com/compozy/compozy/cli`
*   **Dependencies:** `engine`, `pkg/corelibs`
*   **Responsibilities:**
    *   Parsing command-line arguments and flags.
    *   Implementing commands like `dev`, `build`, `deploy`, `init`.
    *   Interacting with the `engine` module to perform actions.
*   **Structure:**
    *   `main.go`: Entry point for the CLI.
    *   `commands/`: Individual CLI command implementations.

### 3. Runtime Package Module (`runtimepkg/`)

This module encapsulates the Deno runtime environment used for executing agent and tool scripts.

*   **Module Path (example):** `github.com/compozy/compozy/runtimepkg`
*   **Dependencies:** `pkg/corelibs` (specifically for NATS communication from Deno to the engine).
*   **Responsibilities:**
    *   Providing the Deno execution environment.
    *   Managing Deno dependencies (`deno.json`, `deno.lock`).
    *   IPC communication layer between the Go engine and Deno scripts via NATS.
*   **Structure:**
    *   Contains the Deno project files (e.g., `runtime.ts`, `src/`, `tests/`).
    *   May have a minimal `go.mod` if Go-based utilities are ever needed to manage or interact with the Deno part from Go.

### 4. Core Libraries Module (`pkg/corelibs/`)

This module groups foundational libraries that provide common services like logging, NATS communication, and templating. These libraries are designed to be used by other Compozy modules.

*   **Module Path (example):** `github.com/compozy/compozy/pkg/corelibs`
*   **Responsibilities:**
    *   Providing a standardized logging facility.
    *   Managing NATS client connections, server setup, and the Compozy-specific NATS protocol.
    *   Offering a template rendering engine.
*   **Key Internal Packages (within `pkg/corelibs/`):**
    *   `logger/`: Logging infrastructure (e.g., wrapper around `charmbracelet/log`). For wider reusability, Compozy-specific styling might need to be made configurable.
    *   `nats/`: NATS client, Compozy-specific protocol definitions, and embedded server management. The protocol layer is currently tailored to Compozy's internal events but the base client/server utilities could be generic.
    *   `tplengine/`: Template rendering engine (e.g., wrapper around `sprig`). For wider reusability, Compozy-specific context preprocessing might need to be made configurable.
*   **Note on Genericity:** While packaged for reuse within Compozy, making these libraries truly generic for external projects might require further refactoring to decouple Compozy-specific configurations or conventions.

### 5. Server Module (`server/`)

This module is responsible for exposing an HTTP API for external systems to interact with Compozy. It could be an optional module or its functionality could be integrated more directly into the `engine` module if a separate API server process isn't desired.

*   **Module Path (example):** `github.com/compozy/compozy/server`
*   **Dependencies:** `engine`, `pkg/corelibs`
*   **Responsibilities:**
    *   Handling incoming HTTP requests.
    *   Implementing API routes for workflow execution, status checking, etc.
    *   Authentication and authorization for API endpoints.
    *   Interacting with the `engine` module.
*   **Structure:**
    *   `main.go`: Entry point for the API server.
    *   `router.go`, `middleware.go`, `handlers/`, etc.

### 6. Build/Development Tools (`cmd/`)

Tools used for development and build processes that are not part of the core runtime engine or CLI application itself.

*   **Example:** `cmd/schemagen/` for generating JSON schemas from Go structs.
*   These might remain as simple command applications at the workspace root or be organized into their own small utility modules if they grow in complexity or require specific Go module dependencies.

## Benefits of this Modular Architecture

*   **Improved Separation of Concerns:** Each module has a well-defined responsibility.
*   **Enhanced Maintainability:** Changes within one module are less likely to impact others directly, assuming clear API contracts between modules.
*   **Scalability:** Individual modules can potentially be scaled or deployed independently if needed in the future.
*   **Independent Development:** Teams could (in theory, for larger projects) work on different modules more independently.
*   **Clearer Dependency Management:** Each module defines its own dependencies via `go.mod`.
*   **Testability:** Modules can be tested in isolation more easily.

This modular structure, combined with a domain-driven approach for internal package organization within the `engine` module, provides a robust foundation for the Compozy Workflow Engine.