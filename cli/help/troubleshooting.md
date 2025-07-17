# Troubleshooting Guide

This guide helps you diagnose and resolve common issues with the Compozy CLI.

## Debug Mode

Always start troubleshooting by enabling debug mode to get detailed information:

```bash
# Enable debug mode for any command
compozy [command] --debug

# Examples
compozy dev --debug
compozy workflow run my-workflow --debug
compozy auth login --debug --interactive
```

## Common Issues

### 1. Configuration Problems

#### Symptom: "Configuration not found" or "Invalid configuration"

```
Error: failed to load configuration: configuration file not found
```

**Solutions:**

```bash
# Check if configuration file exists
ls -la compozy.yaml

# Validate configuration syntax
compozy config validate --debug

# Use explicit config file path
compozy dev --config ./path/to/config.yaml --debug

# Show current configuration sources
compozy config show --debug
```

#### Symptom: Environment variables not being recognized

```
Error: database connection failed
```

**Solutions:**

```bash
# Check environment variables
env | grep COMPOZY_

# Debug configuration loading
compozy config show --debug

# Verify environment variable names (must be uppercase with underscores)
export COMPOZY_DB_HOST=localhost # ✅ Correct
export compozy_db_host=localhost # ❌ Wrong case
```

### 2. Server Connection Issues

#### Symptom: "Connection refused" or "Server not reachable"

```
Error: failed to connect to server at http://localhost:5000
```

**Solutions:**

```bash
# Check if server is running
curl http://localhost:5000/health

# Verify server URL configuration
compozy config show --debug | grep server_url

# Use different server URL
compozy workflow list --server-url https://api.compozy.dev --debug

# Check for port conflicts
netstat -an | grep :5000
lsof -i :5000
```

#### Symptom: "Authentication failed" or "Unauthorized"

```
Error: authentication failed: invalid API key
```

**Solutions:**

```bash
# Check authentication status
compozy auth status --debug

# Re-authenticate
compozy auth login --interactive

# Verify API key
echo $COMPOZY_API_KEY

# Use explicit API key
compozy workflow list --api-key YOUR_API_KEY --debug
```

### 3. Development Server Issues

#### Symptom: "Port already in use"

```
Error: failed to start server: address already in use
```

**Solutions:**

```bash
# Check what's using the port
lsof -i :5000
netstat -an | grep :5000

# Use different port
compozy dev --port 5001

# Kill process using the port (if safe)
kill -9 $(lsof -t -i:5000)

# Let Compozy find available port automatically
compozy dev # Will automatically find next available port
```

#### Symptom: File watcher not working

```
Files changed but server didn't restart
```

**Solutions:**

```bash
# Enable debug mode to see file watcher activity
compozy dev --debug --watch

# Check if files are in ignored directories
ls -la .git node_modules .idea .vscode

# Verify file extensions (only .yaml and .yml are watched)
find . -name "*.yaml" -o -name "*.yml"

# Check file permissions
ls -la compozy.yaml
```

### 4. Workflow Execution Issues

#### Symptom: "Workflow not found"

```
Error: workflow 'my-workflow' not found
```

**Solutions:**

```bash
# List available workflows
compozy workflow list --debug

# Check workflow configuration file
compozy config validate --debug

# Verify workflow definition syntax
cat workflows/my-workflow.yaml

# Check for typos in workflow name
compozy workflow list --format json | jq '.workflows[].name'
```

#### Symptom: "Task execution failed"

```
Error: task execution timeout
```

**Solutions:**

```bash
# Increase timeout
compozy dev --tool-execution-timeout 300s

# Check task configuration
compozy workflow status my-workflow --debug --format json

# Review task logs
compozy workflow logs my-workflow --debug

# Verify tool dependencies
which node npm bun
```

### 5. Output and Display Issues

#### Symptom: Garbled or missing colors

```
Output shows ANSI codes or no colors
```

**Solutions:**

```bash
# Disable colors explicitly
compozy workflow list --no-color

# Force color mode
export COMPOZY_COLOR_MODE=on
compozy workflow list

# Check terminal support
echo $TERM
tput colors
```

#### Symptom: "Invalid output format"

```
Error: unknown output format 'invalid'
```

**Solutions:**

```bash
# Use valid format options
compozy workflow list --format json
compozy workflow list --format tui
compozy workflow list --format auto

# Check current format setting
compozy config show --debug | grep format
```

### 6. Permission and Access Issues

#### Symptom: "Permission denied" errors

```
Error: permission denied: cannot write to file
```

**Solutions:**

```bash
# Check file permissions
ls -la compozy.yaml
chmod 644 compozy.yaml

# Check directory permissions
ls -la .
chmod 755 .

# Run with appropriate user permissions
sudo compozy dev # Only if necessary

# Check disk space
df -h .
```

#### Symptom: "Cannot create config directory"

```
Error: failed to create config directory: permission denied
```

**Solutions:**

```bash
# Check home directory permissions
ls -la ~/.compozy
mkdir -p ~/.compozy
chmod 755 ~/.compozy

# Use local config directory
mkdir -p .compozy
compozy config init --config .compozy/config.yaml
```

## Diagnostic Commands

### System Information

```bash
# Check Compozy version
compozy version

# Check system information
uname -a
which compozy
echo $PATH

# Check Go version (if building from source)
go version
```

### Configuration Diagnostics

```bash
# Show all configuration sources
compozy config show --debug

# Validate configuration
compozy config validate --debug

# Test configuration loading
compozy dev --config ./test-config.yaml --debug --help
```

### Network Diagnostics

```bash
# Test server connectivity
curl -v http://localhost:5000/health

# Check DNS resolution
nslookup api.compozy.dev
dig api.compozy.dev

# Test API endpoints
curl -v -H "Authorization: Bearer $COMPOZY_API_KEY" \
  https://api.compozy.dev/api/v0/workflows
```

### Process Diagnostics

```bash
# Check running processes
ps aux | grep compozy

# Check network connections
netstat -an | grep compozy
lsof -p $(pgrep compozy)

# Check resource usage
top -p $(pgrep compozy)
```

## Getting More Help

### Verbose Logging

```bash
# Enable verbose logging for all operations
export COMPOZY_DEBUG=true
export COMPOZY_LOG_LEVEL=debug

# Run command with maximum verbosity
compozy dev --debug --log-level debug --log-source
```

### Log Files

```bash
# Check system logs (Linux)
journalctl -u compozy
tail -f /var/log/compozy.log

# Check application logs
tail -f ~/.compozy/logs/compozy.log
```

### Environment Information

```bash
# Collect environment information for bug reports
echo "=== System Information ==="
uname -a
echo "=== Compozy Version ==="
compozy version
echo "=== Environment Variables ==="
env | grep COMPOZY_ | sort
echo "=== Configuration ==="
compozy config show --debug
echo "=== Network ==="
curl -I http://localhost:5000/health 2>&1 || echo "Server not reachable"
```

## Common Error Messages

### "context deadline exceeded"

- **Cause**: Operation timeout
- **Solution**: Increase timeout or check network connectivity

```bash
compozy workflow run my-workflow --tool-execution-timeout 300s
```

### "yaml: unmarshal errors"

- **Cause**: Invalid YAML syntax in configuration
- **Solution**: Validate YAML syntax

```bash
compozy config validate --debug
yamllint compozy.yaml
```

### "bind: address already in use"

- **Cause**: Port conflict
- **Solution**: Use different port or stop conflicting process

```bash
compozy dev --port 5001
```

### "no such file or directory"

- **Cause**: Missing configuration or workflow files
- **Solution**: Check file paths and permissions

```bash
ls -la compozy.yaml
find . -name "*.yaml" -type f
```

### "connection refused"

- **Cause**: Server not running or wrong URL
- **Solution**: Start server or verify URL

```bash
compozy dev &
compozy workflow list --server-url http://localhost:5000
```

## Performance Issues

### Slow Startup

```bash
# Profile startup time
time compozy dev --help

# Reduce configuration complexity
compozy config validate --debug

# Check for large configuration files
ls -lah *.yaml
```

### High Memory Usage

```bash
# Monitor memory usage
top -p $(pgrep compozy)

# Adjust memory limits
export GOMAXPROCS=2
compozy dev --max-total-content-size 5242880
```

### File Watcher Performance

```bash
# Reduce watched directories
echo "node_modules" >> .gitignore
echo ".next" >> .gitignore

# Check number of watched files
find . -name "*.yaml" -o -name "*.yml" | wc -l
```

## Reporting Issues

When reporting issues, include:

1. **Compozy version**: `compozy version`
2. **Operating system**: `uname -a`
3. **Command that failed**: Full command with flags
4. **Error message**: Complete error output
5. **Debug output**: Run with `--debug` flag
6. **Configuration**: `compozy config show --debug` (remove sensitive data)
7. **Environment**: `env | grep COMPOZY_`

### Issue Template

```
**Compozy Version:**
compozy version output

**Operating System:**
uname -a output

**Command:**
compozy dev --debug

**Error:**
Full error message

**Configuration:**
compozy config show --debug output (sensitive data removed)

**Steps to Reproduce:**
1. Run command X
2. See error Y
3. Expected Z to happen

**Additional Context:**
Any other relevant information
```
