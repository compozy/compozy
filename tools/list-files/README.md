# @compozy/tool-list-files

A Compozy tool for listing files in a directory.

## Description

This tool provides a simple way to list all files in a specified directory with optional glob pattern exclusion. It returns only files (not subdirectories) and handles errors gracefully by returning an empty array.

## Installation

```bash
npm install @compozy/tool-list-files
```

Or add it to your Compozy workflow configuration.

## API

### `listFiles(input: ListFilesInput): Promise<ListFilesOutput>`

Lists all files in the specified directory.

#### Input

```typescript
interface ListFilesInput {
  dir: string; // The directory path to list files from
  exclude?: string | string[]; // Optional glob pattern(s) to exclude files
}
```

#### Output

```typescript
interface ListFilesOutput {
  files: string[]; // Array of file names found in the directory
}
```

## Usage

### TypeScript

```typescript
import { listFiles } from "@compozy/tool-list-files";

// List all files in a directory
const result = await listFiles({ dir: "./src" });
console.log(result.files);
// Output: ['index.ts', 'utils.ts', 'config.ts', ...]

// Exclude test files
const noTests = await listFiles({
  dir: "./src",
  exclude: "*.test.ts",
});
console.log(noTests.files);
// Output: ['index.ts', 'utils.ts', 'config.ts', ...] (no test files)

// Exclude multiple patterns
const filtered = await listFiles({
  dir: "./src",
  exclude: ["*.test.ts", "*.spec.ts", "*.d.ts"],
});

// Use brace expansion
const noReactFiles = await listFiles({
  dir: "./src",
  exclude: "*.{jsx,tsx}",
});

// Handle non-existent directory
const emptyResult = await listFiles({ dir: "./does-not-exist" });
console.log(emptyResult.files);
// Output: []
```

### Compozy Workflow

```yaml
name: list-project-files
version: 1.0.0
description: List all files in the project source directory

tools:
  - name: list-files
    source: "@compozy/tool-list-files"

tasks:
  - name: list-src-files
    tool: list-files
    input:
      dir: "./src"
      exclude: "*.test.ts" # Exclude test files
    output: src_files

  - name: list-config-files
    tool: list-files
    input:
      dir: "./"
      exclude:
        - "*.test.*"
        - "*.spec.*"
        - "node_modules"
    output: config_files

  - name: display-results
    type: basic
    run: |
      echo "Source files (excluding tests):"
      echo "{{ .src_files.files | join \", \" }}"
      echo "Config files:"
      echo "{{ .config_files.files | join \", \" }}"
```

## Features

- **Non-recursive**: Only lists files in the specified directory, not in subdirectories
- **Glob pattern exclusion**: Exclude files matching specified glob patterns
- **Multiple exclusion patterns**: Support for single pattern or array of patterns
- **Brace expansion**: Support patterns like `*.{js,ts}` to match multiple extensions
- **Case-sensitive matching**: Patterns are case-sensitive by default
- **Error handling**: Returns empty array on any error (non-existent directory, permission issues)
- **Sorted output**: Files are returned in alphabetical order
- **File types**: Includes regular files and symbolic links, excludes directories
- **Hidden files**: Includes hidden files (files starting with .)
- **Special characters**: Handles file names with spaces and special characters

## Error Handling

The tool is designed to be resilient and always returns a valid response:

- **Non-existent directory**: Returns `{ files: [] }`
- **Permission errors**: Returns `{ files: [] }`
- **Invalid input**: Returns `{ files: [] }`
- **Any other errors**: Returns `{ files: [] }`

## Development

### Setup

```bash
# Install dependencies
bun install

# Run tests
bun test

# Run tests in watch mode
bun test --watch

# Type check
bun run typecheck

# Lint
bun run lint
```

### Testing

The tool includes comprehensive tests for:

- Successfully listing files
- Handling empty directories
- Handling non-existent directories
- Handling permission errors (Unix-like systems)
- Invalid input handling
- Hidden files inclusion
- Sorted output verification
- Special characters in file names
- Excluding directories from results
- Symbolic links handling (Unix-like systems)
- Single glob pattern exclusion
- Multiple glob pattern exclusion
- Wildcard pattern matching
- Brace expansion in patterns
- Empty exclusion patterns
- Case-sensitive pattern matching

## License

BSL-1.1
