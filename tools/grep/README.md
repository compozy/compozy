# @compozy/tool-grep

A Compozy tool for searching content in files using pattern matching with full regex support.

## Overview

The grep tool allows you to search for patterns in files and directories, similar to the Unix `grep` command. It supports regular expressions, case-insensitive matching, recursive directory searching, and result limiting.

## Installation

```bash
npm install @compozy/tool-grep
```

## Usage

### Basic Usage

```typescript
import grep from '@compozy/tool-grep';

// Search for a simple pattern in a file
const result = await grep({
  pattern: 'TODO',
  path: './src/index.ts'
});

// Result structure:
// {
//   matches: [
//     {
//       file: '/absolute/path/to/src/index.ts',
//       line: 42,
//       column: 5,
//       text: '// TODO: Implement this feature',
//       lineNumber: 42
//     }
//   ]
// }
```

### Input Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| `pattern` | string | Yes | - | The search pattern (supports regex) |
| `path` | string | Yes | - | File or directory path to search |
| `recursive` | boolean | No | false | Search recursively in subdirectories |
| `ignoreCase` | boolean | No | false | Case-insensitive search |
| `maxResults` | number | No | undefined | Limit the number of results returned |

### Output Format

The tool returns either a success response with matches or an error response:

#### Success Response
```typescript
interface GrepOutput {
  matches: Array<{
    file: string;      // Absolute path to the file
    line: number;      // Line number (1-based)
    column: number;    // Column number (1-based)
    text: string;      // The line content (trimmed)
    lineNumber: number; // Same as line (for compatibility)
  }>;
}
```

#### Error Response
```typescript
interface GrepError {
  error: string;  // Error message
  code?: string;  // Error code (e.g., 'ENOENT', 'EACCES')
}
```

## Examples

### Case-Insensitive Search

```typescript
const result = await grep({
  pattern: 'error',
  path: './logs',
  ignoreCase: true,
  recursive: true
});
```

### Using Regular Expressions

```typescript
// Find all function declarations
const result = await grep({
  pattern: 'function\\s+\\w+\\s*\\(',
  path: './src',
  recursive: true
});

// Find email addresses
const result = await grep({
  pattern: '[\\w._%+-]+@[\\w.-]+\\.[A-Za-z]{2,}',
  path: './data/contacts.txt'
});

// Find TODO or FIXME comments
const result = await grep({
  pattern: '(TODO|FIXME):\\s*.+',
  path: './src',
  recursive: true
});
```

### Common Regex Patterns

#### Words and Text
- `\\bword\\b` - Match whole word only
- `\\w+` - Match one or more word characters
- `\\s+` - Match one or more whitespace characters
- `[A-Za-z]+` - Match alphabetic characters
- `[0-9]+` or `\\d+` - Match digits

#### Lines and Anchors
- `^pattern` - Match at start of line
- `pattern$` - Match at end of line
- `^$` - Match empty lines

#### Special Characters
- `.` - Match any character (except newline)
- `\\.` - Match literal dot
- `\\?` - Match literal question mark
- `\\*` - Match literal asterisk
- `\\[` - Match literal bracket

#### Quantifiers
- `*` - Zero or more occurrences
- `+` - One or more occurrences
- `?` - Zero or one occurrence
- `{n}` - Exactly n occurrences
- `{n,}` - n or more occurrences
- `{n,m}` - Between n and m occurrences

#### Groups and Alternatives
- `(pattern)` - Capture group
- `(?:pattern)` - Non-capturing group
- `pattern1|pattern2` - Match either pattern

### Limiting Results

```typescript
// Get only the first 10 matches
const result = await grep({
  pattern: 'console\\.log',
  path: './src',
  recursive: true,
  maxResults: 10
});
```

### Searching in a Single File vs Directory

```typescript
// Search in a single file
const fileResult = await grep({
  pattern: 'import',
  path: './src/index.ts'
});

// Search in a directory (non-recursive)
const dirResult = await grep({
  pattern: 'import',
  path: './src'
});

// Search in a directory (recursive)
const recursiveResult = await grep({
  pattern: 'import',
  path: './src',
  recursive: true
});
```

## Error Handling

The tool handles various error conditions gracefully:

- **Invalid regex**: Returns error with code `INVALID_REGEX`
- **File not found**: Returns error with code `ENOENT`
- **Permission denied**: Returns error with code `EACCES`
- **Binary files**: Automatically skipped (no matches returned)
- **Unreadable files**: Silently skipped

```typescript
const result = await grep({
  pattern: '[invalid(regex',
  path: './file.txt'
});

if ('error' in result) {
  console.error(`Error: ${result.error}`);
  if (result.code) {
    console.error(`Code: ${result.code}`);
  }
} else {
  console.log(`Found ${result.matches.length} matches`);
}
```

## Notes

- The tool automatically handles binary files by skipping them
- Line and column numbers are 1-based (not 0-based)
- The `text` field in matches contains the trimmed line content
- Regular expressions use JavaScript regex syntax
- The global flag is automatically added to regex patterns
- Permission errors on individual files during directory searches are silently skipped
- The tool resolves relative paths to absolute paths in the output

## License

MIT