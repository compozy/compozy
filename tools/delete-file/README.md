# @compozy/tool-delete-file

A Compozy tool for deleting files from the filesystem.

## Overview

The `delete-file` tool provides a safe way to delete files with proper error handling. It returns a success indicator rather than throwing errors for common scenarios like non-existent files or permission issues.

## Installation

This tool is part of the Compozy ecosystem and can be used within Compozy workflows.

## Usage

### In Workflows

```yaml
name: cleanup-temp-files
version: 1.0.0

tasks:
  - id: delete-temp-file
    type: tool
    tool: delete-file
    input:
      path: "/tmp/temporary-file.txt"
```

### In TypeScript

```typescript
import deleteFile from '@compozy/tool-delete-file';

const result = await deleteFile({
  path: '/path/to/file.txt'
});

if (result.success) {
  console.log('File deleted successfully');
} else {
  console.log('File could not be deleted');
}
```

## Input

| Field | Type   | Required | Description                    |
|-------|--------|----------|--------------------------------|
| path  | string | Yes      | The path to the file to delete |

## Output

| Field   | Type    | Description                                       |
|---------|---------|---------------------------------------------------|
| success | boolean | `true` if file was deleted, `false` otherwise    |

## Error Handling

The tool handles common errors gracefully:

- **Non-existent files**: Returns `{ success: false }` instead of throwing
- **Permission errors**: Returns `{ success: false }` when lacking permissions
- **Invalid input**: Throws an error for missing or invalid path parameter
- **Unexpected errors**: Re-throws for debugging

## Examples

### Basic Usage

```yaml
tasks:
  - id: cleanup
    type: tool
    tool: delete-file
    input:
      path: "./output/temp.txt"
```

### With Conditional Logic

```yaml
tasks:
  - id: delete-old-log
    type: tool
    tool: delete-file
    input:
      path: "./logs/old.log"
  
  - id: check-deletion
    type: conditional
    condition: "{{ .delete-old-log.success }}"
    then:
      - id: log-success
        type: log
        message: "Old log file deleted successfully"
    else:
      - id: log-failure
        type: log
        message: "Could not delete old log file"
```

### Batch Deletion

```yaml
tasks:
  - id: delete-files
    type: map
    items:
      - "./temp/file1.txt"
      - "./temp/file2.txt"
      - "./temp/file3.txt"
    task:
      type: tool
      tool: delete-file
      input:
        path: "{{ .item }}"
```

## Notes

- The tool trims whitespace from the file path
- Returns `false` rather than throwing for expected errors
- Safe to use with non-existent files (idempotent)
- Follows Compozy's standard tool interface