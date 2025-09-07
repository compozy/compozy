# `cli` – _Command-line interface for the Compozy workflow orchestration engine_

> **Provides a unified CLI tool for managing, running, and debugging Compozy projects in development and production environments.**

---

## 📑 Table of Contents

- [🎯 Overview](#-overview)
- [💡 Motivation](#-motivation)
- [⚡ Design Highlights](#-design-highlights)
- [🚀 Getting Started](#-getting-started)
- [📖 Usage](#-usage)
- [🔧 Configuration](#-configuration)
- [🎨 Examples](#-examples)
- [📚 API Reference](#-api-reference)
- [🧪 Testing](#-testing)
- [📦 Contributing](#-contributing)
- [📄 License](#-license)

---

## 🎯 Overview

The `cli` package provides the command-line interface for Compozy, a Next-level Agentic Orchestration Platform. It includes commands for:

- **Development server** (`dev`) - Hot-reloading development environment
- **Configuration management** (`config`) - Inspect, validate, and diagnose configuration
- **MCP proxy** (`mcp-proxy`) - HTTP proxy for Model Context Protocol servers

The CLI integrates with the `pkg/config` package to provide a unified configuration system supporting YAML files, environment variables, and command-line flags with proper precedence handling.

---

## 💡 Motivation

- **Unified Development Experience** - Single command to start development with hot-reloading
- **Configuration Transparency** - Clear visibility into configuration sources and values
- **Production Ready** - MCP proxy for production deployment of AI agent workflows
- **Developer Productivity** - Comprehensive diagnostics and validation tools

---

## ⚡ Design Highlights

- **Cobra Framework** - Professional CLI with subcommands and rich help
- **Hot Reload** - File watcher with intelligent debouncing for development
- **Configuration Precedence** - CLI flags > YAML > Environment > Defaults
- **Port Management** - Automatic port detection and conflict resolution
- **Security** - Input validation and path traversal protection
- **Logging Integration** - Structured logging with configurable levels

---

## 🚀 Getting Started

### Installation

The CLI is included in the main Compozy binary:

```bash
# Install from source
go install github.com/compozy/compozy@latest

# Or build locally
git clone https://github.com/compozy/compozy
cd compozy
go build -o compozy .
```

### Quick Start

```bash
# Start development server
compozy dev

# Start with specific configuration
compozy dev --config custom.yaml --port 5001

# Enable file watching for auto-restart
compozy dev --watch

# Start MCP proxy
compozy mcp-proxy --port 5002
```

---

## 📖 Usage

### CLI Commands

The CLI provides three main commands:

#### Development Server (`dev`)

```bash
compozy dev [flags]
```

Starts the Compozy development server with hot-reloading support.

**Key Features:**

- Auto-restart on configuration changes
- Port conflict resolution
- Environment file loading
- Debug mode support

**Common Options:**

```bash
--config string      Configuration file path (default: compozy.yaml)
--port int          Server port (default: 3000)
--watch             Enable file watching
--debug             Enable debug logging
--env-file string   Environment file path (default: .env)
```

#### Configuration Management (`config`)

```bash
compozy config [show | validate | diagnostics] [flags]
```

Manage and inspect configuration settings.

**Subcommands:**

- `show` - Display current configuration values
- `validate` - Validate configuration files
- `diagnostics` - Run comprehensive configuration diagnostics

#### MCP Proxy (`mcp-proxy`)

```bash
compozy mcp-proxy [flags]
```

Run an HTTP proxy for Model Context Protocol servers.

**Features:**

- Admin API for client management
- IP-based access control
- Structured logging

---

## 🔧 Configuration

### Configuration Sources

The CLI supports multiple configuration sources with precedence:

1. **CLI flags** (highest precedence)
2. **YAML configuration file**
3. **Environment variables**
4. **Default values** (lowest precedence)

### Environment Variables

```bash
# Server configuration
SERVER_HOST=localhost
SERVER_PORT=3000
SERVER_CORS_ENABLED=true

# Database configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=compozy
DB_PASSWORD=secret
DB_NAME=compozy_dev

# Runtime configuration
RUNTIME_LOG_LEVEL=debug
RUNTIME_ENVIRONMENT=development
```

### YAML Configuration

```yaml
# compozy.yaml
server:
  host: localhost
  port: 3000
  cors_enabled: true

database:
  host: localhost
  port: 5432
  user: compozy
  name: compozy_dev

runtime:
  log_level: info
  environment: development
```

---

## 🎨 Examples

### Basic Development

```bash
# Start development server
compozy dev

# Custom configuration
compozy dev --config production.yaml --port 5001

# Debug mode with file watching
compozy dev --debug --watch --log-json
```

### Configuration Management

```bash
# Show current configuration
compozy config show

# Show as JSON
compozy config show --format json

# Validate configuration
compozy config validate --config compozy.yaml

# Run diagnostics
compozy config diagnostics --verbose
```

### MCP Proxy

```bash
# Start MCP proxy
compozy mcp-proxy --port 6001
```

### Advanced Usage

```bash
# Production-like development
compozy dev \
  --config production.yaml \
  --env-file .env.production \
  --log-level info \
  --log-json

# Database override
compozy dev \
  --db-host production.db.example.com \
  --db-port 5432 \
  --db-ssl-mode require
```

---

## 📚 API Reference

### Root Command

```go
func RootCmd() *cobra.Command
```

Returns the root command with all subcommands configured.

### Development Command

```go
func DevCmd() *cobra.Command
```

Creates the development server command with comprehensive flag support.

### Configuration Command

```go
func ConfigCmd() *cobra.Command
```

Provides configuration management subcommands.

### MCP Proxy Command

```go
func MCPProxyCmd() *cobra.Command
```

Creates the MCP proxy server command.

### Helper Functions

```go
func GetConfigCWD(cmd *cobra.Command) (string, string, error)
```

Extracts and resolves working directory and configuration file paths.

---

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./cli/...

# Run specific test
go test -v ./cli -run TestDevCmd

# Run with coverage
go test -coverprofile=coverage.out ./cli/...
go tool cover -html=coverage.out
```

### Test Coverage

The CLI package includes tests for:

- Command configuration and flag parsing
- Configuration loading and validation
- File watching and hot-reload functionality
- Port conflict resolution
- Security validation

### Integration Testing

```bash
# Test with real configuration
compozy config validate --config test/fixtures/compozy-test.yaml

# Test development server
compozy dev --config test/fixtures/compozy-test.yaml --port 0
```

---

## 📦 Contributing

See [CONTRIBUTING.md](../CONTRIBUTING.md) for development guidelines.

### Development Setup

```bash
# Clone repository
git clone https://github.com/compozy/compozy
cd compozy

# Install dependencies
go mod download

# Run tests
make test

# Build CLI
go build -o compozy .
```

### Code Style

- Follow Go conventions and project coding standards
- Use structured logging with context
- Include comprehensive error handling
- Add unit tests for new functionality

---

## 📄 License

BSL-1.1 License - see [LICENSE](../LICENSE) for details.
