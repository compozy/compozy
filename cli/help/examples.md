# Usage Examples

This guide provides common usage patterns and examples for the Compozy CLI, demonstrating how to effectively use global flags across different scenarios.

## Development Workflows

### Starting Development Server

```bash
# Basic development server
compozy dev

# With debug output for troubleshooting
compozy dev --debug

# Using custom configuration
compozy dev --config ./dev.yaml --debug

# With file watching and verbose logging
compozy dev --debug --format tui
```

### Development with Custom Configuration

```bash
# Use staging configuration for development
compozy dev --config ./configs/staging.yaml --debug

# Override specific server settings
compozy dev --server-url http://localhost:8080 --format json
```

## Workflow Management

### Interactive Workflow Operations

```bash
# List workflows with interactive TUI
compozy workflow list --format tui

# Get workflow status with formatted output
compozy workflow status my-workflow --format tui

# Run workflow with interactive feedback
compozy workflow run my-workflow --interactive
```

### Automated Workflow Operations

```bash
# Get workflow status for scripting (JSON output, no colors)
compozy workflow status my-workflow --format json --no-color --quiet

# Run workflow in automation mode
compozy workflow run my-workflow --format json --quiet

# List all workflows for CI/CD pipeline
compozy workflow list --format json --quiet | jq '.workflows[].name'
```

## Authentication

### Interactive Authentication

```bash
# Login with interactive prompts
compozy auth login --interactive

# Login with specific server
compozy auth login --server-url https://api.compozy.dev --interactive

# Check authentication status
compozy auth status --format tui
```

### Automated Authentication

```bash
# Check auth status for scripts
compozy auth status --format json --quiet

# Login with API key from environment
export COMPOZY_API_KEY="your-api-key"
compozy auth status --quiet --format json
```

## Configuration Management

### Working with Different Environments

```bash
# Development environment
compozy dev --config ./configs/development.yaml --debug

# Staging environment
compozy workflow run --config ./configs/staging.yaml --format json

# Production deployment
compozy workflow run production-deploy \
  --config ./configs/production.yaml \
  --format json \
  --quiet \
  --no-color
```

### Configuration Validation

```bash
# Validate configuration with debug output
compozy config validate --config ./my-config.yaml --debug

# Check configuration values
compozy config show --format json
```

## Automation and Scripting

### Bash Script Example

```bash
#!/bin/bash
set -e

# Configuration
export COMPOZY_QUIET=true
export COMPOZY_DEFAULT_FORMAT=json
export COMPOZY_NO_COLOR=true

CONFIG_FILE="./production.yaml"

# Function to check workflow status
check_workflow_status() {
  local workflow_name=$1
  local status

  status=$(compozy workflow status "$workflow_name" --config "$CONFIG_FILE" | jq -r '.status')
  echo "$status"
}

# Function to wait for workflow completion
wait_for_workflow() {
  local workflow_name=$1
  local timeout=${2:-300} # 5 minutes default
  local elapsed=0

  echo "Waiting for workflow '$workflow_name' to complete..."

  while [ $elapsed -lt $timeout ]; do
    local status
    status=$(check_workflow_status "$workflow_name")

    case "$status" in
      "completed")
        echo "Workflow completed successfully"
        return 0
        ;;
      "failed")
        echo "Workflow failed"
        return 1
        ;;
      "running")
        echo "Workflow still running... (${elapsed}s elapsed)"
        sleep 10
        elapsed=$((elapsed + 10))
        ;;
      *)
        echo "Unknown workflow status: $status"
        return 1
        ;;
    esac
  done

  echo "Timeout waiting for workflow completion"
  return 1
}

# Main deployment script
main() {
  echo "Starting production deployment..."

  # Start the workflow
  local workflow_id
  workflow_id=$(compozy workflow run production-deploy --config "$CONFIG_FILE" | jq -r '.id')

  if [ -z "$workflow_id" ] || [ "$workflow_id" = "null" ]; then
    echo "Failed to start workflow"
    exit 1
  fi

  echo "Started workflow with ID: $workflow_id"

  # Wait for completion
  if wait_for_workflow "$workflow_id"; then
    echo "Deployment successful!"
  else
    echo "Deployment failed!"
    exit 1
  fi
}

# Run the script
main "$@"
```

### PowerShell Script Example

```powershell
# PowerShell deployment script
param(
    [string]$Environment = "staging",
    [string]$ConfigPath = "./configs",
    [switch]$Debug
)

# Set environment variables
$env:COMPOZY_QUIET = "true"
$env:COMPOZY_DEFAULT_FORMAT = "json"
$env:COMPOZY_NO_COLOR = "true"

$ConfigFile = Join-Path $ConfigPath "$Environment.yaml"

function Get-WorkflowStatus {
    param([string]$WorkflowName)

    $result = compozy workflow status $WorkflowName --config $ConfigFile
    $status = ($result | ConvertFrom-Json).status
    return $status
}

function Wait-ForWorkflow {
    param(
        [string]$WorkflowName,
        [int]$TimeoutSeconds = 300
    )

    $elapsed = 0
    Write-Host "Waiting for workflow '$WorkflowName' to complete..."

    while ($elapsed -lt $TimeoutSeconds) {
        $status = Get-WorkflowStatus $WorkflowName

        switch ($status) {
            "completed" {
                Write-Host "Workflow completed successfully"
                return $true
            }
            "failed" {
                Write-Host "Workflow failed"
                return $false
            }
            "running" {
                Write-Host "Workflow still running... ($elapsed seconds elapsed)"
                Start-Sleep 10
                $elapsed += 10
            }
            default {
                Write-Host "Unknown workflow status: $status"
                return $false
            }
        }
    }

    Write-Host "Timeout waiting for workflow completion"
    return $false
}

# Main execution
try {
    Write-Host "Deploying to $Environment environment..."

    $flags = @("--config", $ConfigFile)
    if ($Debug) { $flags += "--debug" }

    $result = compozy workflow run "deploy-$Environment" @flags
    $workflowId = ($result | ConvertFrom-Json).id

    if (-not $workflowId) {
        throw "Failed to start workflow"
    }

    Write-Host "Started workflow with ID: $workflowId"

    if (Wait-ForWorkflow $workflowId) {
        Write-Host "Deployment successful!" -ForegroundColor Green
    } else {
        throw "Deployment failed!"
    }
}
catch {
    Write-Error $_.Exception.Message
    exit 1
}
```

## CI/CD Integration

### GitHub Actions

```yaml
name: Deploy with Compozy
on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Install Compozy CLI
        run: |
          curl -sSL https://install.compozy.dev | bash

      - name: Deploy to Production
        env:
          COMPOZY_API_KEY: ${{ secrets.COMPOZY_API_KEY }}
          COMPOZY_QUIET: true
          COMPOZY_DEFAULT_FORMAT: json
          COMPOZY_NO_COLOR: true
        run: |
          compozy workflow run production-deploy \
            --config ./production.yaml \
            --server-url https://api.compozy.dev
```

### GitLab CI

```yaml
stages:
  - deploy

deploy_production:
  stage: deploy
  image: alpine:latest
  before_script:
    - apk add --no-cache curl jq
    - curl -sSL https://install.compozy.dev | sh
  script:
    - |
      compozy workflow run production-deploy \
        --config ./production.yaml \
        --format json \
        --quiet \
        --no-color
  variables:
    COMPOZY_API_KEY: $CI_COMPOZY_API_KEY
    COMPOZY_SERVER_URL: https://api.compozy.dev
  only:
    - main
```

## Debugging and Troubleshooting

### Debug Mode Examples

```bash
# Full debug output for development server
compozy dev --debug --format tui

# Debug workflow execution
compozy workflow run my-workflow --debug --format json

# Debug authentication issues
compozy auth login --debug --interactive

# Debug configuration loading
compozy config validate --debug --config ./my-config.yaml
```

### Verbose Logging

```bash
# Combine debug with specific output format
compozy workflow status my-workflow --debug --format json

# Debug with custom configuration
compozy dev --debug --config ./debug.yaml --format tui
```

## Output Format Examples

### JSON Output for Parsing

```bash
# Get workflow list as JSON
compozy workflow list --format json | jq '.workflows[].name'

# Get detailed workflow status
compozy workflow status my-workflow --format json | jq '{name: .name, status: .status, duration: .duration}'

# Extract specific field
WORKFLOW_STATUS=$(compozy workflow status my-workflow --format json --quiet | jq -r '.status')
```

### TUI Output for Interactive Use

```bash
# Interactive workflow management
compozy workflow list --format tui

# Interactive development server with debug
compozy dev --format tui --debug

# Interactive authentication
compozy auth login --format tui --interactive
```
