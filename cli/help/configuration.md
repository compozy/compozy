# Configuration Guide

Compozy CLI uses a flexible configuration system that supports multiple sources with clear precedence rules. This guide explains how to configure the CLI for different environments and use cases.

## Configuration Sources

Configuration values are resolved in the following order (highest to lowest priority):

1. **CLI Flags** - Direct command-line arguments
2. **Config File** - Values from YAML configuration file
3. **Environment Variables** - Environment variable values
4. **Defaults** - Built-in default values

## Configuration File

### Default Location

The CLI looks for configuration files in this order:

1. File specified with `--config` flag
2. `compozy.yaml` in current directory
3. `compozy.yml` in current directory
4. `.compozy/config.yaml` in current directory
5. `~/.compozy/config.yaml` in home directory

### Basic Configuration Structure

```yaml
# compozy.yaml
cli:
  server_url: "https://api.compozy.dev"
  default_format: "tui"
  debug: false
  quiet: false
  interactive: false
  color_mode: "auto"

server:
  host: "localhost"
  port: 5000
  cors_enabled: true

database:
  host: "localhost"
  port: 5432
  name: "compozy_dev"
  ssl_mode: "disable"

temporal:
  host: "localhost:7233"
  namespace: "compozy-dev"
  task_queue: "compozy-tasks"

runtime:
  log_level: "info"
  tool_execution_timeout: "60s"
```

## CLI Configuration Section

The `cli` section controls CLI behavior and global flag defaults:

```yaml
cli:
  # Server connection
  server_url: "https://api.compozy.dev"
  config_file: "./custom-config.yaml"

  # Output control
  default_format: "tui" # json, tui, auto
  color_mode: "auto" # auto, on, off

  # Behavior control
  debug: false # Enable debug mode
  quiet: false # Suppress non-essential output
  interactive: false # Force interactive mode
```

## Environment Variables

All configuration values can be set via environment variables using the prefix `COMPOZY_` and uppercase with underscores:

### CLI Environment Variables

```bash
# Server connection
export COMPOZY_SERVER_URL="https://api.compozy.dev"
export COMPOZY_CONFIG_FILE="./production.yaml"

# Output control
export COMPOZY_DEFAULT_FORMAT="json"
export COMPOZY_COLOR_MODE="off"

# Behavior control
export COMPOZY_DEBUG="true"
export COMPOZY_QUIET="true"
export COMPOZY_INTERACTIVE="false"
```

### Development Server Environment Variables

```bash
# Server configuration
export COMPOZY_SERVER_HOST="0.0.0.0"
export COMPOZY_SERVER_PORT="8080"
export COMPOZY_SERVER_CORS_ENABLED="true"

# Database configuration
export COMPOZY_DB_HOST="localhost"
export COMPOZY_DB_PORT="5432"
export COMPOZY_DB_NAME="compozy_production"
export COMPOZY_DB_USER="compozy"
export COMPOZY_DB_PASSWORD="secret"
export COMPOZY_DB_SSL_MODE="require"

# Temporal configuration
export COMPOZY_TEMPORAL_HOST="temporal.example.com:7233"
export COMPOZY_TEMPORAL_NAMESPACE="production"
export COMPOZY_TEMPORAL_TASK_QUEUE="prod-tasks"

# Runtime configuration
export COMPOZY_RUNTIME_LOG_LEVEL="warn"
export COMPOZY_TOOL_EXECUTION_TIMEOUT="120s"
```

## Environment-Specific Configurations

### Development Configuration

```yaml
# development.yaml
cli:
  debug: true
  default_format: "tui"
  color_mode: "auto"

server:
  host: "localhost"
  port: 5000
  cors_enabled: true

database:
  host: "localhost"
  port: 5432
  name: "compozy_dev"
  ssl_mode: "disable"

runtime:
  log_level: "debug"
  tool_execution_timeout: "300s"
```

### Staging Configuration

```yaml
# staging.yaml
cli:
  server_url: "https://staging-api.compozy.dev"
  debug: false
  default_format: "json"
  quiet: true

server:
  host: "0.0.0.0"
  port: 8080
  cors_enabled: false

database:
  host: "staging-db.internal"
  port: 5432
  name: "compozy_staging"
  ssl_mode: "require"

temporal:
  host: "staging-temporal.internal:7233"
  namespace: "staging"
  task_queue: "staging-tasks"

runtime:
  log_level: "info"
  tool_execution_timeout: "180s"
```

### Production Configuration

```yaml
# production.yaml
cli:
  server_url: "https://api.compozy.dev"
  debug: false
  default_format: "json"
  quiet: true
  color_mode: "off"

server:
  host: "0.0.0.0"
  port: 8080
  cors_enabled: false

database:
  host: "prod-db.internal"
  port: 5432
  name: "compozy_production"
  ssl_mode: "require"

temporal:
  host: "prod-temporal.internal:7233"
  namespace: "production"
  task_queue: "prod-tasks"

runtime:
  log_level: "warn"
  tool_execution_timeout: "120s"

limits:
  max_nesting_depth: 10
  max_string_length: 1048576
  max_message_content: 2097152
  max_total_content_size: 10485760
```

## Configuration Validation

### Validate Configuration

```bash
# Validate default configuration
compozy config validate

# Validate specific configuration file
compozy config validate --config ./production.yaml

# Validate with debug output
compozy config validate --config ./my-config.yaml --debug
```

### Show Current Configuration

```bash
# Show current configuration values
compozy config show

# Show as JSON for parsing
compozy config show --format json

# Show specific section
compozy config show --section cli --format json
```

## Configuration Best Practices

### 1. Use Environment-Specific Files

Keep separate configuration files for each environment:

```
configs/
├── development.yaml
├── staging.yaml
├── production.yaml
└── local.yaml
```

### 2. Use Environment Variables for Secrets

Never store sensitive information in configuration files:

```yaml
# ❌ Bad - secrets in config file
database:
  password: "my-secret-password"

# ✅ Good - use environment variables
database:
  password: "${DB_PASSWORD}"
```

### 3. Layer Configurations

Use a base configuration with environment-specific overrides:

```yaml
# base.yaml
server:
  cors_enabled: false

database:
  ssl_mode: "require"

# development.yaml (inherits from base)
server:
  cors_enabled: true  # Override for development

database:
  ssl_mode: "disable"  # Override for development
```

### 4. Use CLI Flags for Overrides

Use CLI flags for temporary overrides without modifying configuration files:

```bash
# Temporarily enable debug mode
compozy dev --config ./production.yaml --debug

# Temporarily change output format
compozy workflow list --config ./production.yaml --format tui
```

### 5. Document Configuration Changes

Keep a changelog for configuration updates:

```yaml
# production.yaml
# Last updated: 2024-01-15
# Changes:
# - Increased tool_execution_timeout from 60s to 120s
# - Added memory limits configuration
# - Updated temporal namespace to "production"

cli:
  server_url: "https://api.compozy.dev"
  # ... rest of configuration
```

## Configuration Debugging

### Debug Configuration Loading

```bash
# See how configuration is resolved
compozy config show --debug

# Validate configuration with detailed output
compozy config validate --debug --config ./my-config.yaml

# Start dev server with configuration debugging
compozy dev --debug --config ./development.yaml
```

### Common Configuration Issues

#### 1. Invalid YAML Syntax

```bash
# Check for syntax errors
compozy config validate --config ./broken.yaml --debug
```

#### 2. Missing Required Values

```bash
# Identify missing required configuration
compozy config validate --config ./incomplete.yaml --debug
```

#### 3. Environment Variable Substitution

```bash
# Debug environment variable resolution
export COMPOZY_DEBUG=true
compozy config show --debug
```

#### 4. Configuration Precedence Issues

```bash
# See which sources are being used
compozy config show --debug --format json | jq '.sources'
```

## Template Variables

Configuration files support environment variable substitution:

```yaml
# Use environment variables in configuration
database:
  host: "${DB_HOST:-localhost}"
  port: "${DB_PORT:-5432}"
  name: "${DB_NAME:-compozy_dev}"
  user: "${DB_USER:-compozy}"
  password: "${DB_PASSWORD}"

temporal:
  host: "${TEMPORAL_HOST:-localhost:7233}"
  namespace: "${TEMPORAL_NAMESPACE:-compozy-${ENVIRONMENT:-dev}}"
```

Usage:

```bash
export ENVIRONMENT=staging
export DB_HOST=staging-db.internal
export DB_PASSWORD=staging-secret
export TEMPORAL_NAMESPACE=staging

compozy dev --config ./template.yaml
```
