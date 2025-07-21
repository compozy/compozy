# @compozy/tool-list-dir

A directory listing tool for Compozy workflows that provides flexible file and directory enumeration with glob pattern support.

## Features

- List files and directories with detailed metadata
- Support for glob patterns including globstar (`**`)
- Recursive directory traversal
- Filtering by file type (files only, directories only, or both)
- Graceful error handling
- Cross-platform compatibility

## API

### Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `path` | string | Yes | - | The directory path to list |
| `pattern` | string | No | - | Glob pattern for filtering entries |
| `recursive` | boolean | No | false | Enable recursive directory traversal |
| `includeFiles` | boolean | No | true | Include files in the output |
| `includeDirs` | boolean | No | true | Include directories in the output |

### Output Format

Returns an object with an `entries` array containing directory entry objects:

```typescript
interface DirEntry {
  name: string;      // File or directory name
  path: string;      // Full absolute path
  type: 'file' | 'dir';  // Entry type
  size: number;      // Size in bytes (0 for directories)
  modified: string;  // ISO 8601 timestamp
}
```

## Usage Examples

### TypeScript

```typescript
import tool from '@compozy/tool-list-dir';

// Basic directory listing
const result = await tool({
  path: './src'
});
console.log(result.entries);

// List only JavaScript files
const jsFiles = await tool({
  path: './src',
  pattern: '*.js'
});

// Recursive listing with glob pattern
const allTests = await tool({
  path: './src',
  pattern: '**/*.test.ts',
  recursive: true
});

// List only directories
const dirs = await tool({
  path: '.',
  includeFiles: false,
  includeDirs: true
});
```

### Compozy Workflow

```yaml
name: list-project-files
tasks:
  - id: list-source
    type: tool
    tool: list-dir
    input:
      path: ./src
      pattern: "**/*.{js,ts}"
      recursive: true
    
  - id: process-files
    type: basic
    run: |
      echo "Found {{ .tasks.list_source.result.entries | len }} source files"
      {{ range .tasks.list_source.result.entries }}
      echo "- {{ .name }} ({{ .size }} bytes)"
      {{ end }}
```

## Glob Pattern Examples

The tool uses `minimatch` for pattern matching, supporting standard glob syntax:

### Basic Patterns

- `*.txt` - All .txt files in the directory
- `*.{js,ts}` - All .js and .ts files
- `test*` - All files/directories starting with "test"
- `*config*` - All entries containing "config"

### Globstar Patterns (requires `recursive: true`)

- `**/*.js` - All .js files in any subdirectory
- `src/**/*.ts` - All .ts files under the src directory
- `**/test/*.spec.js` - All .spec.js files in any test directory
- `**/{src,lib}/*.js` - All .js files in any src or lib directory

### Advanced Patterns

- `!(*.test).js` - All .js files except test files
- `?(a|b).txt` - a.txt or b.txt (optional)
- `+(a|b).txt` - One or more of a.txt or b.txt
- `@(a|b).txt` - Exactly a.txt or b.txt
- `a/**/*!(.d).ts` - All .ts files under a/ except .d.ts files

## Error Handling

The tool is designed to fail gracefully:

- Non-existent paths return empty results
- Permission errors skip inaccessible entries
- Invalid inputs return empty results
- Broken symlinks are skipped
- All errors return `{ entries: [] }` rather than throwing

## Development

### Installation

```bash
bun install
```

### Running Tests

```bash
bun test
```

### Building

This tool is designed to be executed directly by the Compozy runtime and doesn't require a build step.

## Notes

- Directory entries always have `size: 0`
- Entries are sorted alphabetically by path for consistent output
- Hidden files (starting with `.`) are included by default
- Symbolic links are followed, but broken links are skipped
- When using `recursive: true` with patterns, directories are traversed even if they don't match the pattern (to find matching files within)