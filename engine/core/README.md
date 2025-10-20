# `core` – _Core Types and Utilities for Compozy Engine_

> **Provides fundamental types, interfaces, and utilities that form the foundation of the Compozy workflow orchestration engine.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Library](#library)
  - [Configuration Interface](#configuration-interface)
  - [ID Generation](#id-generation)
  - [Type System](#type-system)
  - [File Loading](#file-loading)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `core` package provides the foundational types, interfaces, and utilities that underpin the entire Compozy workflow orchestration engine. It defines the common contracts and shared functionality used across all engine components.

This package handles:

- Core type definitions and constants
- Configuration interface contracts
- Unique identifier generation
- File loading and path resolution
- Status and event type definitions
- Memory reference structures
- Environment variable management
- Utility functions for serialization and deep copying

---

## 💡 Motivation

- **Shared Foundation**: Provide consistent types and interfaces across all engine components
- **Type Safety**: Ensure type-safe operations throughout the system
- **Configuration Contracts**: Define standard interfaces for all configuration types
- **Identifier Management**: Provide secure, unique identifier generation
- **Path Security**: Safe file path resolution and validation
- **Extensibility**: Enable easy extension of core functionality

---

## ⚡ Design Highlights

- **Interface-Driven Design**: All configurations implement the `Config` interface for consistency
- **Type-Safe Enums**: Strongly-typed constants for statuses, events, and component types
- **Secure ID Generation**: Uses KSUID for collision-resistant, sortable identifiers
- **Path Validation**: Built-in path resolution with security checks
- **Generic Utilities**: Type-safe serialization and deep copying functions
- **Error Handling**: Structured error types with context and suggestions
- **Memory Management**: Comprehensive memory reference system

---

## 🚀 Getting Started

```go
import (
    "context"
    "github.com/compozy/compozy/engine/core"
)

// Generate unique IDs
id, err := NewID()
if err != nil {
    log.Fatal(err)
}
fmt.Printf("Generated ID: %s\n", id)

// Create working directory context
cwd, err := CWDFromPath("/path/to/project")
if err != nil {
    log.Fatal(err)
}

// Load configuration
config, _, err := LoadConfig[*MyConfig]("/path/to/config.yaml")
if err != nil {
    log.Fatal(err)
}

// Validate configuration
if err := config.Validate(); err != nil {
    log.Fatal(err)
}
```

---

## 📖 Usage

### Library

#### Basic Type Usage

```go
// Generate unique identifiers
id := MustNewID()
fmt.Printf("ID: %s\n", id.String())

// Work with status types
status := StatusRunning
fmt.Printf("Status: %s\n", status)

// Convert proto status to core status
coreStatus := ToStatus("WORKFLOW_STATUS_RUNNING")
fmt.Printf("Core Status: %s\n", coreStatus)

// Get version information
version := GetVersion()
fmt.Printf("Version: %s\n", version)
```

#### Working with Paths

```go
// Create working directory context
cwd, err := CWDFromPath("/path/to/project")
if err != nil {
    log.Fatal(err)
}

// Resolve relative paths safely
resolved, err := ResolvePath(cwd, "configs/agent.yaml")
if err != nil {
    log.Fatal(err)
}

// Validate and join paths
safePath, err := cwd.JoinAndCheck("subdir/file.yaml")
if err != nil {
    log.Fatal(err)
}
```

### Configuration Interface

#### Implementing the Config Interface

```go
type MyConfig struct {
    ID           string            `json:"id"`
    Name         string            `json:"name"`
    Settings     map[string]any    `json:"settings"`

    filePath     string
    cwd          *PathCWD
}

// Implement Config interface
func (c *MyConfig) Component() ConfigType {
    return ConfigTool // or appropriate type
}

func (c *MyConfig) SetFilePath(path string) {
    c.filePath = path
}

func (c *MyConfig) GetFilePath() string {
    return c.filePath
}

func (c *MyConfig) SetCWD(path string) error {
    cwd, err := CWDFromPath(path)
    if err != nil {
        return err
    }
    c.cwd = cwd
    return nil
}

func (c *MyConfig) GetCWD() *PathCWD {
    return c.cwd
}

func (c *MyConfig) GetEnv() EnvMap {
    return EnvMap{} // Return environment variables
}

func (c *MyConfig) GetInput() *Input {
    return &Input{} // Return input parameters
}

func (c *MyConfig) Validate() error {
    if c.ID == "" {
        return fmt.Errorf("id is required")
    }
    return nil
}

func (c *MyConfig) ValidateInput(ctx context.Context, input *Input) error {
    // Validate input parameters
    return nil
}

func (c *MyConfig) ValidateOutput(ctx context.Context, output *Output) error {
    // Validate output parameters
    return nil
}

func (c *MyConfig) HasSchema() bool {
    return false
}

func (c *MyConfig) Merge(other any) error {
    // Implement merge logic
    return nil
}

func (c *MyConfig) AsMap() (map[string]any, error) {
    return AsMapDefault(c)
}

func (c *MyConfig) FromMap(data any) error {
    config, err := FromMapDefault[*MyConfig](data)
    if err != nil {
        return err
    }
    return c.Merge(config)
}
```

### ID Generation

#### Safe ID Generation

```go
// Generate new ID with error handling
id, err := NewID()
if err != nil {
    log.Fatal(err)
}

// Generate ID that panics on error (for initialization)
id := MustNewID()

// IDs are sortable by creation time
id1 := MustNewID()
time.Sleep(1 * time.Millisecond)
id2 := MustNewID()

// id1 < id2 lexicographically
fmt.Printf("ID1: %s\n", id1)
fmt.Printf("ID2: %s\n", id2)
```

### Type System

#### Working with Status Types

```go
// Core status types
statuses := []StatusType{
    StatusPending,
    StatusRunning,
    StatusSuccess,
    StatusFailed,
    StatusTimedOut,
    StatusCanceled,
    StatusWaiting,
    StatusPaused,
}

// Convert from proto statuses
protoStatus := "WORKFLOW_STATUS_RUNNING"
coreStatus := ToStatus(protoStatus)
fmt.Printf("Core Status: %s\n", coreStatus)

// Event types
events := []EvtType{
    EvtDispatched,
    EvtStarted,
    EvtSuccess,
    EvtFailed,
    EvtCanceled,
}

// Component types
components := []ComponentType{
    ComponentWorkflow,
    ComponentTask,
    ComponentAgent,
    ComponentTool,
}
```

### File Loading

#### Configuration Loading

```go
// Load typed configuration
config, path, err := LoadConfig[*MyConfig]("/path/to/config.yaml")
if err != nil {
    log.Fatal(err)
}

// Load as generic map
configMap, err := MapFromFilePath("/path/to/config.yaml")
if err != nil {
    log.Fatal(err)
}
```

---

## 🔧 Configuration

### Core Types

#### StatusType

Represents execution status across the system.

```go
const (
    StatusPending  StatusType = "PENDING"
    StatusRunning  StatusType = "RUNNING"
    StatusSuccess  StatusType = "SUCCESS"
    StatusFailed   StatusType = "FAILED"
    StatusTimedOut StatusType = "TIMED_OUT"
    StatusCanceled StatusType = "CANCELED"
    StatusWaiting  StatusType = "WAITING"
    StatusPaused   StatusType = "PAUSED"
)
```

#### ComponentType

Identifies different component types in the system.

```go
const (
    ComponentWorkflow ComponentType = "workflow"
    ComponentTask     ComponentType = "task"
    ComponentAgent    ComponentType = "agent"
    ComponentTool     ComponentType = "tool"
    ComponentLog      ComponentType = "log"
)
```

#### ConfigType

Identifies configuration types for the autoloader.

```go
const (
    ConfigProject  ConfigType = "project"
    ConfigWorkflow ConfigType = "workflow"
    ConfigTask     ConfigType = "task"
    ConfigAgent    ConfigType = "agent"
    ConfigTool     ConfigType = "tool"
    ConfigMCP      ConfigType = "mcp"
    ConfigMemory   ConfigType = "memory"
)
```

### Memory Reference System

#### MemoryReference

Defines how components access memory resources.

```go
type MemoryReference struct {
    ID          string `yaml:"id" json:"id" validate:"required"`
    Mode        string `yaml:"mode,omitempty" json:"mode,omitempty" validate:"omitempty,oneof=read-write read-only"`
    Key         string `yaml:"key" json:"key" validate:"required"`
    ResolvedKey string `yaml:"-" json:"-"`
}
```

---

## 🎨 Examples

### Complete Configuration Implementation

```go
package mypackage

import (
    "context"
    "fmt"
    "github.com/compozy/compozy/engine/core"
    "github.com/compozy/compozy/engine/schema"
)

type MyToolConfig struct {
    Resource     string                 `json:"resource,omitempty" yaml:"resource,omitempty"`
    ID           string                 `json:"id" yaml:"id" validate:"required"`
    Name         string                 `json:"name" yaml:"name"`
    Runtime      string                 `json:"runtime" yaml:"runtime" validate:"required"`
    Code         string                 `json:"code" yaml:"code"`
    With         *Input           `json:"with,omitempty" yaml:"with,omitempty"`
    Env          *EnvMap          `json:"env,omitempty" yaml:"env,omitempty"`
    InputSchema  *schema.Schema        `json:"input,omitempty" yaml:"input,omitempty"`
    OutputSchema *schema.Schema        `json:"output,omitempty" yaml:"output,omitempty"`

    filePath     string
    cwd          *PathCWD
}

func (c *MyToolConfig) Component() ConfigType {
    return ConfigTool
}

func (c *MyToolConfig) SetFilePath(path string) {
    c.filePath = path
}

func (c *MyToolConfig) GetFilePath() string {
    return c.filePath
}

func (c *MyToolConfig) SetCWD(path string) error {
    cwd, err := CWDFromPath(path)
    if err != nil {
        return err
    }
    c.cwd = cwd
    return nil
}

func (c *MyToolConfig) GetCWD() *PathCWD {
    return c.cwd
}

func (c *MyToolConfig) GetEnv() EnvMap {
    if c.Env == nil {
        return EnvMap{}
    }
    return *c.Env
}

func (c *MyToolConfig) GetInput() *Input {
    if c.With == nil {
        return &Input{}
    }
    return c.With
}

func (c *MyToolConfig) Validate() error {
    validator := schema.NewCompositeValidator(
        schema.NewStructValidator(c),
        schema.NewCWDValidator(c.cwd, c.ID),
    )
    return validator.Validate()
}

func (c *MyToolConfig) ValidateInput(ctx context.Context, input *Input) error {
    return schema.NewParamsValidator(input, c.InputSchema, c.ID).Validate(ctx)
}

func (c *MyToolConfig) ValidateOutput(ctx context.Context, output *Output) error {
    return schema.NewParamsValidator(output, c.OutputSchema, c.ID).Validate(ctx)
}

func (c *MyToolConfig) HasSchema() bool {
    return c.InputSchema != nil || c.OutputSchema != nil
}

func (c *MyToolConfig) Merge(other any) error {
    otherConfig, ok := other.(*MyToolConfig)
    if !ok {
        return fmt.Errorf("cannot merge: incompatible types")
    }

    // Implement merge logic
    if otherConfig.Name != "" {
        c.Name = otherConfig.Name
    }
    if otherConfig.Runtime != "" {
        c.Runtime = otherConfig.Runtime
    }

    return nil
}

func (c *MyToolConfig) AsMap() (map[string]any, error) {
    return AsMapDefault(c)
}

func (c *MyToolConfig) FromMap(data any) error {
    config, err := FromMapDefault[*MyToolConfig](data)
    if err != nil {
        return err
    }
    return c.Merge(config)
}

// Usage example
func main() {
    // Create configuration
    config := &MyToolConfig{
        ID:      "file-processor",
        Name:    "File Processing Tool",
        Runtime: "node",
        Code:    "console.log('Processing file...');",
    }

    // Set working directory
    if err := config.SetCWD("/path/to/project"); err != nil {
        log.Fatal(err)
    }

    // Validate configuration
    if err := config.Validate(); err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Configuration valid: %s\n", config.ID)
}
```

### Working with Deep Copying

```go
// Deep copy configurations
original := &MyToolConfig{
    ID:   "original",
    Name: "Original Tool",
    With: &Input{
        "param1": "value1",
        "param2": map[string]any{
            "nested": "value",
        },
    },
}

// Create deep copy
copied, err := DeepCopy(original)
if err != nil {
    log.Fatal(err)
}

// Modify copy without affecting original
copied.Name = "Modified Tool"
(*copied.With)["param1"] = "modified"

fmt.Printf("Original: %s\n", original.Name)  // "Original Tool"
fmt.Printf("Copied: %s\n", copied.Name)     // "Modified Tool"
```

### Error Handling with Context

```go
// Create structured error with context
err := NewError(
    fmt.Errorf("configuration validation failed"),
    "CONFIG_VALIDATION_ERROR",
    map[string]any{
        "config_id":   "my-tool",
        "config_type": "tool",
        "suggestion":  "Check required fields and syntax",
    },
)

// Handle the error
var coreErr *Error
if errors.As(err, &coreErr) {
    fmt.Printf("Error Code: %s\n", coreErr.Code)
    fmt.Printf("Context: %+v\n", coreErr.Context)
}
```

---

## 📚 API Reference

### Core Types

#### `ID`

Unique identifier type using KSUID.

```go
type ID string

func NewID() (ID, error)
func MustNewID() ID
func (c ID) String() string
```

#### `Config`

Interface that all configuration types must implement.

```go
type Config interface {
    Component() ConfigType
    SetFilePath(string)
    GetFilePath() string
    SetCWD(path string) error
    GetCWD() *PathCWD
    GetEnv() EnvMap
    GetInput() *Input
    Validate() error
    ValidateInput(ctx context.Context, input *Input) error
    ValidateOutput(ctx context.Context, output *Output) error
    HasSchema() bool
    Merge(other any) error
    AsMap() (map[string]any, error)
    FromMap(any) error
}
```

#### `MemoryReference`

Memory access configuration.

```go
type MemoryReference struct {
    ID          string `yaml:"id" json:"id" validate:"required"`
    Mode        string `yaml:"mode,omitempty" json:"mode,omitempty"`
    Key         string `yaml:"key" json:"key" validate:"required"`
    ResolvedKey string `yaml:"-" json:"-"`
}
```

### Key Functions

#### File Operations

```go
func ResolvePath(cwd *PathCWD, path string) (string, error)
func LoadConfig[T Config](filePath string) (T, string, error)
func MapFromFilePath(path string) (map[string]any, error)
```

#### Serialization

```go
func AsMapDefault(config any) (map[string]any, error)
func FromMapDefault[T any](data any) (T, error)
func DeepCopy[T any](v T) (T, error)
```

#### Utilities

```go
func GetVersion() string
func GetStoreDir(cwd string) string
func ToStatus(status string) StatusType
```

---

## 🧪 Testing

### Running Tests

```bash
# Run all core package tests
go test -v ./engine/core

# Run specific test
go test -v ./engine/core -run TestNewID

# Run tests with coverage
go test -v ./engine/core -cover

# Run benchmark tests
go test -v ./engine/core -bench=.
```

### Test Structure

The package includes comprehensive tests for:

- ID generation and uniqueness
- Configuration interface implementation
- File loading and path resolution
- Type conversions and serialization
- Error handling and validation

### Example Test

```go
func TestNewID(t *testing.T) {
    // Test ID generation
    id1, err := NewID()
    require.NoError(t, err)
    require.NotEmpty(t, id1)

    id2, err := NewID()
    require.NoError(t, err)
    require.NotEmpty(t, id2)

    // IDs should be unique
    require.NotEqual(t, id1, id2)

    // Test MustNewID
    id3 := MustNewID()
    require.NotEmpty(t, id3)
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
