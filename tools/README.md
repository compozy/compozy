# Compozy Tools

This directory contains reusable tools that can be used in Compozy workflows. Each tool is a separate npm package that follows the `@compozy/tool-*` naming convention.

## Available Tools

### File System Tools

#### @compozy/tool-read-file

Reads file content from the file system.

- **Input**: `path` (string)
- **Output**: `content` (string)

#### @compozy/tool-write-file

Writes content to a file, with optional append mode.

- **Input**: `path` (string), `content` (string), `append` (boolean, optional)
- **Output**: `success` (boolean)

#### @compozy/tool-delete-file

Deletes a file from the file system.

- **Input**: `path` (string)
- **Output**: `success` (boolean)

#### @compozy/tool-list-files

Lists files in a directory (non-recursive, files only).

- **Input**: `dir` (string)
- **Output**: `files` (array of strings)

#### @compozy/tool-list-dir

Lists directory contents with glob filtering and metadata.

- **Input**: `path` (string), `pattern` (string, optional), `recursive` (boolean, optional), `includeFiles` (boolean, optional), `includeDirs` (boolean, optional)
- **Output**: `entries` (array of objects with name, path, type, size, modified)

### Text Processing Tools

#### @compozy/tool-grep

Searches for content in files using regex patterns.

- **Input**: `pattern` (string), `path` (string), `recursive` (boolean, optional), `ignoreCase` (boolean, optional), `maxResults` (number, optional)
- **Output**: `matches` (array of objects with file, line, column, text, lineNumber)

### System Tools

#### @compozy/tool-exec

Executes bash commands with security restrictions.

- **Input**: `command` (string), `cwd` (string, optional), `env` (object, optional), `timeout` (number, optional)
- **Output**: `stdout` (string), `stderr` (string), `exitCode` (number), `success` (boolean)
- **Security**: Only whitelisted commands are allowed

### Network Tools

#### @compozy/tool-fetch

Performs HTTP/HTTPS requests.

- **Input**: `url` (string), `method` (string, optional), `headers` (object, optional), `body` (string|object, optional), `timeout` (number, optional)
- **Output**: `status` (number), `statusText` (string), `headers` (object), `body` (string), `success` (boolean)

## Usage

### In Workflows

Add the tool to your workflow YAML:

```yaml
tools:
  - id: file-reader
    $use: "@compozy/tool-read-file"

tasks:
  read-config:
    type: basic
    tool: file-reader
    input:
      path: "./config.json"
```

### In TypeScript

Import and use directly in your entrypoint:

```typescript
export { default as readFile } from "@compozy/tool-read-file";
export { default as writeFile } from "@compozy/tool-write-file";
export { default as fetch } from "@compozy/tool-fetch";
// ... etc
```

## Development

Each tool follows the same structure:

- `index.ts` - Main tool implementation
- `index.test.ts` - Test suite
- `package.json` - Package configuration
- `tsconfig.json` - TypeScript configuration
- `README.md` - Tool-specific documentation

### Testing

Run tests for a specific tool:

```bash
cd tools/[tool-name]
bun test
```

Run all tool tests:

```bash
bun test tools/
```

## Contributing

When creating new tools:

1. Follow the `@compozy/tool-*` naming convention
2. Export a default async function that accepts input and returns output
3. Include comprehensive error handling
4. Add thorough tests
5. Document all parameters and examples
6. Ensure TypeScript compatibility
7. Follow security best practices
