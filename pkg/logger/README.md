# `logger` – _Structured logging library with beautiful terminal output_

> **A context-aware logging abstraction built on charm.sh/log that provides structured logging with beautiful terminal styling, automatic test detection, and seamless integration with Go's context system.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
  - [Basic Logging](#basic-logging)
  - [Context-Aware Logging](#context-aware-logging)
  - [Structured Logging](#structured-logging)
  - [Gin Middleware](#gin-middleware)
  - [Command Line Integration](#command-line-integration)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `logger` package provides a clean, structured logging interface that wraps the excellent [charmbracelet/log](https://github.com/charmbracelet/log) library. It offers beautiful terminal output with Tailwind CSS-inspired colors, context-aware logging, and automatic test environment detection.

Key features include structured key-value logging, customizable output formats (text or JSON), automatic test environment detection, and seamless integration with Go's context system for request tracing and hierarchical logging.

---

## 💡 Motivation

- **Consistency**: Standardized logging interface across the Compozy codebase
- **Beauty**: Elegant terminal output with carefully chosen Tailwind CSS-inspired colors
- **Context**: Seamless integration with Go's context package for request tracing
- **Testing**: Automatic quiet mode detection for clean test output without manual configuration

---

## ⚡ Design Highlights

- **Context-First Design**: Loggers are stored in and retrieved from context.Context for request correlation
- **Automatic Test Detection**: Intelligently detects test environments and suppresses output
- **Beautiful Styling**: Tailwind CSS-inspired color scheme with consistent formatting and readability
- **Structured Logging**: Key-value pairs for machine-readable logs with special error handling
- **Singleton Pattern**: Thread-safe default logger with lazy initialization
- **Gin Integration**: Middleware for automatic request logging with path tracking

---

## 🚀 Getting Started

```go
package main

import (
    "context"
    "errors"
    "github.com/compozy/compozy/pkg/logger"
)

func main() {
    // Get logger from context (falls back to default)
    ctx := context.Background()
    log := logger.FromContext(ctx)

    // Start logging with structured key-value pairs
    log.Info("Application starting", "version", "1.0.0", "env", "development")
    log.Debug("Debug information", "component", "main", "pid", os.Getpid())
    log.Error("Something went wrong", "err", errors.New("example error"))
}
```

---

## 📖 Usage

### Library

#### Basic Logging

```go
// Create a logger with default configuration
log := logger.NewLogger(nil)

// Different log levels with structured data
log.Debug("Debug message", "module", "auth", "user_id", 123)
log.Info("Info message", "action", "login", "user", "john")
log.Warn("Warning message", "component", "auth", "attempt", 3)
log.Error("Error occurred", "err", err, "operation", "database")
```

#### Context-Aware Logging

```go
// Store logger in context
ctx := logger.ContextWithLogger(context.Background(), log)

// Retrieve logger from context anywhere in your call stack
log := logger.FromContext(ctx)
log.Info("Request processed", "path", "/api/users", "method", "GET")

// Create child loggers with additional context
childLog := log.With("requestId", "abc123", "userId", 42)
childLog.Info("Processing user operation")
childLog.Debug("Database query", "table", "users", "duration", "15ms")
```

#### Structured Logging

```go
log := logger.NewLogger(nil)

// Key-value pairs for structured logging
log.Info("User login",
    "userId", 123,
    "email", "user@example.com",
    "ip", "192.168.1.1",
    "userAgent", "Mozilla/5.0...",
    "timestamp", time.Now().Unix())

// Special handling for errors (highlighted in pink)
log.Error("Database connection failed",
    "err", err,
    "host", "localhost",
    "port", 5432,
    "retryCount", 3)
```

#### Gin Middleware

```go
import (
    "github.com/gin-gonic/gin"
    "github.com/compozy/compozy/pkg/logger"
)

func setupRouter() *gin.Engine {
    r := gin.New()

    // Create logger with configuration
    log := logger.NewLogger(&logger.Config{
        Level: logger.InfoLevel,
        JSON:  false,
        AddSource: false,
    })

    // Add logging middleware - automatically logs requests
    r.Use(logger.Middleware(log))

    r.GET("/api/users", func(c *gin.Context) {
        // Logger is automatically available in context
        log := logger.FromContext(c.Request.Context())
        log.Info("Processing users request", "userAgent", c.GetHeader("User-Agent"))
        c.JSON(200, gin.H{"users": []string{}})
    })

    return r
}
```

#### Command Line Integration

```go
import (
    "github.com/spf13/cobra"
    "github.com/compozy/compozy/pkg/logger"
)

func main() {
    var rootCmd = &cobra.Command{
        Use:   "myapp",
        Short: "My application",
        Run: func(cmd *cobra.Command, args []string) {
            // Get logger configuration from command line flags
            logLevel, logJSON, logSource, err := logger.GetLoggerConfig(cmd)
            if err != nil {
                panic(err)
            }

            // Setup logger with parsed configuration
            log := logger.SetupLogger(logLevel, logJSON, logSource)
            log.Info("Application started", "args", args)
        },
    }

    // Add standard logging flags
    rootCmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error, disabled)")
    rootCmd.PersistentFlags().Bool("log-json", false, "Output logs in JSON format")
    rootCmd.PersistentFlags().Bool("log-source", false, "Include source file and line numbers")

    rootCmd.Execute()
}
```

---

## 🔧 Configuration

### Configuration Structure

```go
type Config struct {
    Level      LogLevel      // debug, info, warn, error, disabled
    Output     io.Writer     // Where to write logs (default: os.Stdout)
    JSON       bool          // Use JSON format instead of text
    AddSource  bool          // Include source file and line numbers
    TimeFormat string        // Time format (default: "15:04:05")
}
```

### Log Levels

```go
const (
    DebugLevel    LogLevel = "debug"
    InfoLevel     LogLevel = "info"
    WarnLevel     LogLevel = "warn"
    ErrorLevel    LogLevel = "error"
    DisabledLevel LogLevel = "disabled"
)
```

### Configuration Examples

```go
// Development configuration
devConfig := &logger.Config{
    Level:      logger.DebugLevel,
    Output:     os.Stdout,
    JSON:       false,
    AddSource:  true,
    TimeFormat: "15:04:05",
}

// Production configuration
prodConfig := &logger.Config{
    Level:      logger.InfoLevel,
    Output:     os.Stdout,
    JSON:       true,
    AddSource:  false,
    TimeFormat: "2006-01-02T15:04:05Z07:00",
}

// Test configuration (automatically used in tests)
testConfig := logger.TestConfig() // Outputs to io.Discard
```

---

## 🎨 Examples

### Complete Application Setup

```go
package main

import (
    "context"
    "net/http"
    "os"
    "time"

    "github.com/gin-gonic/gin"
    "github.com/compozy/compozy/pkg/logger"
)

func main() {
    // Configure logger based on environment
    var config *logger.Config
    if os.Getenv("ENV") == "production" {
        config = &logger.Config{
            Level:      logger.InfoLevel,
            JSON:       true,
            AddSource:  false,
            TimeFormat: time.RFC3339,
        }
    } else {
        config = &logger.Config{
            Level:      logger.DebugLevel,
            JSON:       false,
            AddSource:  true,
            TimeFormat: "15:04:05",
        }
    }

    log := logger.NewLogger(config)

    // Setup Gin with logging middleware
    r := gin.New()
    r.Use(logger.Middleware(log))

    // API endpoint with structured logging
    r.GET("/users/:id", func(c *gin.Context) {
        log := logger.FromContext(c.Request.Context())
        userID := c.Param("id")

        log.Info("Fetching user", "userId", userID, "ip", c.ClientIP())

        // Simulate some processing
        user, err := fetchUser(userID)
        if err != nil {
            log.Error("Failed to fetch user",
                "err", err,
                "userId", userID,
                "duration", time.Since(start))
            c.JSON(500, gin.H{"error": "Internal server error"})
            return
        }

        log.Info("User fetched successfully",
            "userId", userID,
            "userName", user.Name,
            "duration", time.Since(start))
        c.JSON(200, user)
    })

    log.Info("Server starting", "port", 5001, "env", os.Getenv("ENV"))
    http.ListenAndServe(":5001", r)
}
```

### Background Task Logging

```go
func processJobWithLogging(ctx context.Context, jobID string) {
    log := logger.FromContext(ctx).With("jobId", jobID, "component", "jobProcessor")

    log.Info("Job started", "timestamp", time.Now().Unix())

    defer func() {
        if r := recover(); r != nil {
            log.Error("Job panicked", "panic", r, "stack", debug.Stack())
        }
    }()

    // Process job steps with detailed logging
    for i, step := range steps {
        stepLog := log.With("step", i, "stepName", step.Name)
        stepLog.Debug("Step starting", "input", step.Input)

        start := time.Now()
        if err := step.Execute(); err != nil {
            stepLog.Error("Step failed",
                "err", err,
                "duration", time.Since(start),
                "attempt", step.Attempt)
            return
        }

        stepLog.Info("Step completed",
            "duration", time.Since(start),
            "output", step.Output)
    }

    log.Info("Job completed successfully", "totalDuration", time.Since(jobStart))
}
```

### Hierarchical Logging

```go
func handleRequest(ctx context.Context) {
    log := logger.FromContext(ctx)

    // Create service-specific logger
    serviceLog := log.With("service", "userService")

    // Further nest for specific operations
    authLog := serviceLog.With("operation", "authenticate")
    authLog.Info("Authenticating user", "method", "oauth")

    // Database operation logging
    dbLog := serviceLog.With("operation", "database")
    dbLog.Debug("Querying users table", "query", "SELECT * FROM users")

    // Response logging
    responseLog := serviceLog.With("operation", "response")
    responseLog.Info("Sending response", "status", 200, "size", 1024)
}
```

---

## 📚 API Reference

### Core Types

```go
type Logger interface {
    Debug(msg string, keyvals ...any)
    Info(msg string, keyvals ...any)
    Warn(msg string, keyvals ...any)
    Error(msg string, keyvals ...any)
    With(args ...any) Logger
}

type LogLevel string
type ContextKey string

const (
    LoggerCtxKey ContextKey = "logger"
    DebugLevel   LogLevel   = "debug"
    InfoLevel    LogLevel   = "info"
    WarnLevel    LogLevel   = "warn"
    ErrorLevel   LogLevel   = "error"
    NoLevel      LogLevel   = ""
    DisabledLevel LogLevel  = "disabled"
)
```

### Factory Functions

```go
func NewLogger(cfg *Config) Logger
func NewForTests() Logger
func SetupLogger(lvl LogLevel, json, source bool) Logger
```

### Context Functions

```go
func ContextWithLogger(ctx context.Context, l Logger) context.Context
func FromContext(ctx context.Context) Logger
```

### Gin Integration

```go
func Middleware(log Logger) gin.HandlerFunc
```

### Command Line Integration

```go
func GetLoggerConfig(cmd *cobra.Command) (LogLevel, bool, bool, error)
```

### Configuration

```go
func DefaultConfig() *Config
func TestConfig() *Config
func IsTestEnvironment() bool
```

### Styling

The logger uses Tailwind CSS-inspired colors:

- **Debug**: Emerald-500 (`#10b981`)
- **Info**: Indigo-500 (`#6366f1`)
- **Warn**: Amber-500 (`#f59e0b`)
- **Error**: Pink-500 (`#ec4899`)
- **Fatal**: Red-500 (`#ef4444`)

---

## 🧪 Testing

The logger automatically detects test environments and uses quiet configuration:

```go
func TestMyFunction(t *testing.T) {
    ctx := context.Background()
    log := logger.FromContext(ctx) // Automatically uses TestConfig()

    // This won't produce output during tests
    log.Info("Test message", "testCase", "basic")

    // Verify logger behavior
    assert.NotNil(t, log)
}

// For explicit test logger
func TestWithExplicitLogger(t *testing.T) {
    log := logger.NewForTests()
    log.Info("This is silent in tests")

    // Test with custom configuration
    testConfig := &logger.Config{
        Level:  logger.DebugLevel,
        Output: &bytes.Buffer{}, // Capture output
        JSON:   false,
    }
    log = logger.NewLogger(testConfig)
}
```

### Test Detection

The logger detects test environments by checking:

- Command line arguments ending in `.test`
- Environment variables `GO_TEST=1` or `TESTING=1`
- Binary names containing `___` (Go test binaries)

### Benchmarking

```go
func BenchmarkLogger(b *testing.B) {
    log := logger.NewForTests()

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        log.Info("Benchmark message", "iteration", i)
    }
}
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../../CONTRIBUTING.md)

---

## 📄 License

BSL-1.1 License - see [LICENSE](../../LICENSE)
