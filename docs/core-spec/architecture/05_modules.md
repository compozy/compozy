# Modules Architecture

## Overview

The Compozy Workflow Engine is designed with a well-defined package structure, allowing for easy extension and customization of its components. This document outlines the high-level architecture based on a single main Go module and domain-driven design principles for the internal structure of its key packages. This approach aims to enhance maintainability, scalability, and clarity across the codebase.

The system is fundamentally event-driven, with components communicating via NATS messages (Commands and State Events) as detailed in the Core Spec (see `docs/core-spec/events/`). The Go packages described below implement the conceptual components outlined in `docs/core-spec/architecture/02_components.md`. Event structures are defined using Protocol Buffers (Protobuf), with `.proto` files located in the `proto/` directory and generated Go code residing in `pkg/pb/`.

## Project Structure

```text
compozy-go/
├── go.mod                       // Defines the main Go module (e.g., github.com/compozy/compozy)
├── go.sum
├── main.go                      // Main application entry point (CLI)
│
├── proto/                       // Protocol Buffer definitions for events
│   ├── common/
│   ├── workflow/
│   ├── task/
│   ├── agent/
│   └── tool/
│
├── engine/                      // Core engine logic package
│   ├── common/                  // Shared utilities and types for the engine
│   ├── domain/                  // Domain-specific logic
│   │   ├── agent/
│   │   ├── project/
│   │   ├── task/
│   │   ├── tool/
│   │   ├── trigger/
│   │   └── workflow/
│   └── schema/                  // JSON schema definitions and validation for configs
│
├── cli/                         // CLI commands package
│   ├── root.go
│   ├── dev.go
│   ├── build.go
│   ├── deploy.go
│   └── init.go
│
├── pkg/                         // Shared utility packages
│   ├── pb/                      // Generated Go code from Protobuf definitions
│   │   ├── common/
│   │   ├── workflow/
│   │   ├── task/
│   │   ├── agent/
│   │   └── tool/
│   ├── logger/                  // Logging utilities
│   ├── nats/                    // NATS client, server, and protocol wrapper
│   ├── runtime/                 // Deno runtime environment for tools/agents
│   │   ├── deno.json
│   │   ├── src/                 // TypeScript source for Deno runtime
│   │   └── ...
│   ├── schemagen/               // JSON schema generation logic for configs
│   │   └── main.go              // Runnable for generating schemas
│   ├── tplengine/               // Template engine
│   └── utils/                   // Common utilities
│
├── server/                      // HTTP API server package
│   ├── server.go
│   ├── router.go
│   ├── middleware.go
│   └── ...
│
├── docs/
│   └── ...
├── examples/
│   └── ...
├── schemas/                     // Output directory for generated JSON schemas (for YAML configs)
│   └── ...
├── scripts/
│   └── ...
├── test/
│   └── ...
│
├── .gitignore
├── Makefile
...
```

The Compozy project is organized as a single Go module (e.g., `github.com/compozy/compozy` as defined in `go.mod`). This module contains several top-level packages, each responsible for a specific set of functionalities, promoting separation of concerns.

The primary packages and key directories are:

1.  **Engine Package (`engine/`)**: The core of the Compozy Workflow Engine.
2.  **CLI Package (`cli/`)**: The command-line interface.
3.  **Proto Definitions (`proto/`)**: Contains `.proto` files defining the structure of NATS events.
4.  **Shared Packages (`pkg/`)**: Contains reusable packages including `pb/` (generated Protobuf code), `runtime` (for Deno), `logger`, `nats`, etc.
5.  **Server Package (`server/`)**: Handles the HTTP API.
6.  **Root `main.go`**: The main entry point for the CLI application.

## Mapping Conceptual Components to Packages

The conceptual components defined in `docs/core-spec/architecture/02_components.md` are implemented across the Go package structure as follows:

*   **API Service (`api.Service`)**: Implemented by the `server/` package. It handles external HTTP requests and produces commands (defined in `proto/` and compiled to `pkg/pb/`) like `WorkflowTrigger`.
*   **System Orchestrator (`system.Orchestrator`)**: Core logic resides within the `engine/` package. It consumes commands from the `API Service` and `Task Executor`, and produces `WorkflowExecute` commands. All event data structures are sourced from `pkg/pb/`.
*   **State Manager (`state.Manager`)**: Logic resides within the `engine/` package. It consumes all state events (e.g., `WorkflowExecutionStarted`, `TaskExecutionFailed`, defined in `proto/` and compiled to `pkg/pb/`) for persistence, likely leveraging `pkg/nats` for JetStream capabilities or another database via a repository pattern.
*   **Workflow Executor (`workflow.Executor`)**: Implemented within `engine/domain/workflow/` and related parts of the `engine/` package. It consumes `WorkflowExecute` commands and produces workflow state events and `TaskExecute` commands (using `pkg/pb/` types).
*   **Task Executor (`task.Executor`)**: Implemented within `engine/domain/task/` and related parts of the `engine/` package. It consumes `TaskExecute` commands and produces task state events, as well as `AgentExecute` and `ToolExecute` commands for the `System Runtime` (using `pkg/pb/` types).
*   **System Runtime (`system.Runtime`)**: Implemented by the `pkg/runtime/` package (Deno environment). It consumes `AgentExecute` and `ToolExecute` commands from the `Task Executor` (via NATS IPC using `pkg/pb/` types for serialization) and produces agent/tool execution state events.
*   **System Monitoring (`system.Monitoring`)**: This is a conceptual component. Its functionality will be realized by collecting and processing `LogEmitted` events (defined in `proto/` and compiled to `pkg/pb/`) and other state events produced by various packages. Log aggregation and metrics exposure might leverage tools external to the core application, fed by the structured logs from `pkg/logger` and events from `pkg/nats`.
*   **NATS Client (`nats.Client`)**: Implemented by the `pkg/nats/` package, providing the communication backbone. It will handle serialization/deserialization of Protobuf messages defined in `pkg/pb/`.
*   **NATS Server (`nats.Server`)**: Potentially managed by `pkg/nats/` for embedded use during development or deployed as a standalone service.

## Key Packages & Directories Detailed

### 1. Proto Definitions (`proto/`)

This directory houses all Protocol Buffer (`.proto`) definition files. These files define the contract for all event messages (Commands and State Events) exchanged between components via NATS.

*   **Responsibilities:**
    *   Define the structure, fields, and types for all NATS messages.
    *   Organized into subdirectories mirroring the event domains (e.g., `workflow`, `task`, `agent`, `tool`, `common`).
*   **Output:** Go code is generated from these `.proto` files into the `pkg/pb/` directory by `protoc` (typically via a `make protos` command).

### 2. Engine Package (`engine/`)

This is the central package containing the primary business logic of the Compozy Workflow Engine. It implements core conceptual components like the `System Orchestrator`, `Workflow Executor`, `Task Executor`, and `State Manager`.

*   **Package Path:** `github.com/compozy/compozy/engine`
*   **Dependencies:** `pkg/*` (including `pkg/pb/` for event types, `pkg/logger`, `pkg/nats`, `pkg/runtime` for dispatching)
*   **Responsibilities:**
    *   Orchestration of workflows, tasks, agents, and tools using event types from `pkg/pb/`.
    *   Parsing, validation, and interpretation of all Compozy YAML definition files (contents defined by `engine/schema/` JSON Schemas).
    *   Management of execution state, including persistence via the `State Manager` concept.
    *   Driving the lifecycle of workflows and tasks by consuming and producing NATS commands and state events (Protobuf messages).
    *   Coordinating with `pkg/runtime` for the execution of tools and agents.
*   **Key Internal Sub-Packages (within `engine/`):**
    *   `common/`: Shared utilities, types (e.g., CWD, parameters, references), and base configurations for the engine.
    *   `domain/`: Contains sub-packages for each core domain entity (`agent/`, `task/`, `tool/`, `workflow/`, `project/`, `trigger/`) handling their specific logic, YAML configuration, and validation.
    *   `schema/`: JSON schema definitions and validation logic used by domain packages for YAML configuration files.

### 3. CLI Package (`cli/`)

This package provides the command-line interface. The `main.go` in the project root is the application entry point, typically invoking the CLI.

*   **Package Path:** `github.com/compozy/compozy/cli`
*   **Dependencies:** `engine/`, `pkg/*`, `server/` (for the `dev` command)
*   **Responsibilities:**
    *   Parsing command-line arguments (Cobra).
    *   Implementing commands: `dev` (starts API server, NATS, and engine components), `build`, `deploy`, `init`.
    *   Interacting with `engine/` and `server/` packages.
*   **Structure:**
    *   `root.go`: Defines the root Cobra command.
    *   Individual command files (e.g., `dev.go`, `build.go`).

### 4. Shared Packages (`pkg/`)

Contains reusable utility packages.

*   **`pkg/pb/`**: Contains the Go struct definitions generated by `protoc` from the `.proto` files in `proto/`. These structs represent the NATS events and are used throughout the application for type-safe event handling.
*   **`pkg/logger/`**: Provides standardized logging. Produces `LogEmitted` conceptual events (defined in `proto/log/` and compiled to `pkg/pb/log/`).
*   **`pkg/nats/`**: Implements the `NATS Client` and manages an optional embedded `NATS Server`. Defines the Compozy NATS protocol, handling the serialization and deserialization of Protobuf messages (from `pkg/pb/`) for all inter-component communication.
*   **`pkg/runtime/`**: Implements the `System Runtime` for Deno.
    *   **Responsibilities:** Executes agent and tool TypeScript code. Communicates with the `engine/` via NATS, consuming `AgentExecute`/`ToolExecute` commands (Protobuf messages) and producing their respective state events (Protobuf messages). Manages Deno dependencies (`deno.json`).
*   **`pkg/schemagen/`**: Generates JSON schemas (for validating YAML configuration files) from Go structs in `engine/domain/` and `engine/common/`.
*   **`pkg/tplengine/`**: Template rendering engine.
*   **`pkg/utils/`**: Common utilities.

### 5. Server Package (`server/`)

Implements the `API Service` component.

*   **Package Path:** `github.com/compozy/compozy/server`
*   **Dependencies:** `engine/`, `pkg/*` (including `pkg/pb/` for command payloads if needed for request validation before sending to NATS)
*   **Responsibilities:**
    *   Handles incoming HTTP requests (Gin), including webhook triggers.
    *   Implements API routes as defined in `docs/core-spec/architecture/04_api_routes.md`.
    *   Constructs and produces NATS commands (using `pkg/pb/` types) to the `engine/` (System Orchestrator).
*   **Structure:**
    *   `server.go`, `router.go`, `middleware.go`, `config.go`, `state.go`, `errors.go`.

### 6. Root `main.go` & Build Artifacts

*   **`main.go` (at project root):** Entry point, initializes and executes the CLI.
*   **Schema Generation (`pkg/schemagen/main.go`):** Runnable Go program (`go run ./pkg/schemagen/main.go` or `make schemagen`) to generate JSON schemas (for YAML config validation) into `schemas/`.
*   **Protobuf Generation (`Makefile` target, e.g., `make protos`):** Command to invoke `protoc` to compile `.proto` files from `proto/` into Go code in `pkg/pb/`.

## Benefits of this Package-Based Architecture

*   **Improved Separation of Concerns:** Each top-level package and conceptual component has a well-defined responsibility.
*   **Enhanced Maintainability:** Changes are localized, respecting clear interfaces (Go package APIs, Protobuf event contracts, and NATS event contracts).
*   **Type Safety:** Protobuf ensures type-safe event structures across components.
*   **Logical Modularity:** The distinct packages provide a modular design within a single Go module.
*   **Clearer Internal Dependencies:** Go's package system and the defined NATS event flows clarify interactions.
*   **Testability:** Packages and conceptual components can be tested more easily in isolation.

This package-based structure, combined with a domain-driven approach for internal organization within the `engine` package, and Protobuf for event definitions, provides a robust foundation for the Compozy Workflow Engine, aligning with the event-driven architecture described in the core specifications.