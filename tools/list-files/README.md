# @compozy/tool-list-files

A Compozy tool for listing files in a directory.

## Description

This tool provides a simple way to list all files in a specified directory. It returns only files (not subdirectories) and handles errors gracefully by returning an empty array.

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
import { listFiles } from '@compozy/tool-list-files';

// List files in a directory
const result = await listFiles({ dir: './src' });
console.log(result.files);
// Output: ['index.ts', 'utils.ts', 'config.ts', ...]

// Handle non-existent directory
const emptyResult = await listFiles({ dir: './does-not-exist' });
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
    output: src_files

  - name: display-results
    type: basic
    run: |
      echo "Files found in src directory:"
      echo "{{ .src_files.files | join \", \" }}"
```

## Features

- **Non-recursive**: Only lists files in the specified directory, not in subdirectories
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

## License

MIT