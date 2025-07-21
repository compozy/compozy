# @compozy/tool-exec

A secure tool for executing bash commands within the Compozy runtime environment.

## Overview

This tool provides a controlled way to execute bash commands with built-in security validation, timeout handling, and environment management. It's designed to work within the Compozy workflow orchestration system.

## Security Features

### Command Validation

The tool implements strict security measures to prevent command injection and malicious operations:

1. **Pattern Blocking**: Blocks dangerous patterns including:
   - Command chaining (`;`, `&&`, `||`)
   - Pipes and redirects (`|`, `>`, `<`)
   - Command substitution (`$()`, backticks)
   - Variable expansion (`${...}`)
   - Line continuation (`\`)

2. **Whitelist Approach**: Only allows execution of pre-approved commands:
   - File operations: `ls`, `cat`, `grep`, `find`, `head`, `tail`, `sort`, `uniq`
   - System info: `pwd`, `whoami`, `hostname`, `date`, `uptime`
   - File info: `file`, `stat`, `basename`, `dirname`, `realpath`
   - Process info: `ps`, `top`, `free`
   - Text processing: `echo`, `wc`, `cut`, `awk`, `sed`
   - Utilities: `which`, `whereis`, `sleep`

3. **Timeout Protection**: 
   - Default timeout: 30 seconds
   - Maximum timeout: 5 minutes
   - Automatic process termination on timeout

## Usage

```typescript
import exec from "@compozy/tool-exec";

// Simple command execution
const result = await exec({
  command: "echo 'Hello, World!'"
});

// With custom working directory
const files = await exec({
  command: "ls -la",
  cwd: "/path/to/directory"
});

// With environment variables
const envResult = await exec({
  command: "echo $MY_VAR",
  env: { MY_VAR: "custom value" }
});

// With custom timeout
const slowCommand = await exec({
  command: "find . -name '*.ts'",
  timeout: 60000 // 60 seconds
});
```

## Input Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `command` | string | Yes | The bash command to execute |
| `cwd` | string | No | Working directory (defaults to current directory) |
| `env` | object | No | Environment variables (merged with process.env) |
| `timeout` | number | No | Timeout in milliseconds (default: 30000, max: 300000) |

## Output Format

### Success Response
```typescript
{
  stdout: string;    // Standard output from the command
  stderr: string;    // Standard error output (may be empty)
  exitCode: number;  // Process exit code (0 for success)
  success: boolean;  // true if exitCode is 0
}
```

### Error Response
```typescript
{
  error: string;     // Error description
  stdout?: string;   // Partial stdout if available
  stderr?: string;   // Partial stderr if available
  exitCode?: number; // Exit code if process started
}
```

## Examples

### List Files
```typescript
const result = await exec({
  command: "ls -la"
});
if ('success' in result && result.success) {
  console.log(result.stdout);
}
```

### Search Files
```typescript
const result = await exec({
  command: "find . -name '*.json' -type f",
  cwd: "/path/to/search",
  timeout: 60000
});
```

### Process Text
```typescript
const result = await exec({
  command: "cat file.txt | grep 'pattern' | wc -l"
});
// Note: This will be rejected due to pipe usage
```

### Safe Alternative for Pipes
Since pipes are blocked for security, use multiple commands:
```typescript
// First, get the file content
const catResult = await exec({
  command: "cat file.txt"
});

// Then process it in your application code
if ('success' in catResult && catResult.success) {
  const lines = catResult.stdout.split('\n');
  const matches = lines.filter(line => line.includes('pattern'));
  console.log(`Found ${matches.length} matches`);
}
```

## Security Warnings

⚠️ **Important Security Considerations:**

1. **Never pass user input directly to commands** - Always validate and sanitize
2. **Use the whitelist** - Only whitelisted commands are allowed
3. **Avoid dynamic command construction** - Prefer static commands with parameters
4. **Check file paths** - Ensure paths are within expected boundaries
5. **Monitor timeouts** - Long-running commands may indicate issues

## Testing

Run the test suite:
```bash
bun test
```

The test suite covers:
- Input validation
- Security validation
- Successful command execution
- Error handling
- Timeout behavior
- Working directory changes
- Environment variable passing

## License

MIT