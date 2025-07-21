# @compozy/tool-write-file

A Compozy tool for writing content to files with support for creating directories and appending content.

## Installation

This tool is part of the Compozy tools collection and is designed to be used within the Compozy runtime environment.

## Usage

### Basic Usage

```typescript
import { writeFile } from "@compozy/tool-write-file";

// Write content to a file
const result = await writeFile({
  path: "output/data.txt",
  content: "Hello, World!"
});

if (result.success) {
  console.log("File written successfully");
} else {
  console.error("Error:", result.error);
}
```

### Appending Content

```typescript
// Append content to an existing file
const result = await writeFile({
  path: "logs/app.log",
  content: "New log entry\n",
  append: true
});
```

### With Configuration

```typescript
// Write with custom encoding and permissions
const result = await writeFile({
  path: "config/settings.json",
  content: JSON.stringify({ key: "value" }, null, 2)
}, {
  encoding: "utf-8",
  mode: 0o600  // Read/write for owner only
});
```

## Input Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| path | string | Yes | The file path to write to (relative paths only) |
| content | string | Yes | The content to write |
| append | boolean | No | Whether to append to existing file (default: false) |

## Configuration Options

| Option | Type | Default | Description |
|--------|------|---------|-------------|
| encoding | BufferEncoding | "utf-8" | File encoding |
| mode | number | 0o644 | File permissions mode |

## Output

The tool returns an object with the following structure:

```typescript
{
  success: boolean;
  error?: string;  // Only present if success is false
}
```

## Features

- **Directory Creation**: Automatically creates parent directories if they don't exist
- **Append Mode**: Option to append content to existing files
- **Error Handling**: Graceful handling of permission errors, disk space issues, and invalid paths
- **Security**: Prevents directory traversal and absolute path access
- **Encoding Support**: Configurable file encoding
- **Permission Control**: Set file permissions on creation

## Error Cases

The tool handles various error scenarios:

- Invalid input (missing or wrong type)
- Permission denied
- Path not found
- Target is a directory
- No space left on device
- Security violations (absolute paths, parent directory references)

## Example in Compozy Workflow

```yaml
tasks:
  - id: save-data
    type: basic
    tool: write_file
    input:
      path: "output/report.txt"
      content: "{{ .data }}"
    
  - id: append-log
    type: basic
    tool: write_file
    input:
      path: "logs/process.log"
      content: "[{{ .timestamp }}] Process completed\n"
      append: true
```

## Testing

Run tests with:

```bash
bun test
```

The test suite covers:
- Successfully writing files
- Appending to existing files
- Creating nested directories
- Handling permission errors
- Input validation
- Security checks
- Various edge cases

## Security Considerations

- The tool only accepts relative paths
- Parent directory references (`..`) are blocked
- Absolute paths are rejected
- Environment variable names in paths are not expanded

## License

MIT