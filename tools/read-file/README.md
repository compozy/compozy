# @compozy/tool-read-file

A Compozy tool for reading file content from the filesystem.

## Installation

This tool is part of the Compozy monorepo and is automatically available when using Compozy.

## Usage

### In a Compozy workflow

```yaml
name: read-config
type: basic
task:
  tool: readFile
  input:
    path: ./config.json
```

### In TypeScript

```typescript
import { readFile } from "@compozy/tool-read-file";

// Read a file
const result = await readFile({
  path: "./example.txt"
});

if ("error" in result) {
  console.error("Error:", result.error);
} else {
  console.log("Content:", result.content);
}
```

## API

### Input

The tool expects an input object with the following structure:

```typescript
interface ReadFileInput {
  path: string; // Path to the file to read
}
```

### Output

The tool returns either a success or error response:

#### Success Response

```typescript
interface ReadFileOutput {
  content: string; // The file content
}
```

#### Error Response

```typescript
interface ReadFileError {
  error: string;   // Error message
  code?: string;   // Error code (e.g., ENOENT, EACCES, EISDIR)
}
```

## Error Handling

The tool handles various error scenarios:

- **File not found** (ENOENT): Returns an error when the specified file doesn't exist
- **Permission denied** (EACCES): Returns an error when the file cannot be read due to permissions
- **Directory instead of file** (EISDIR): Returns an error when the path points to a directory
- **Invalid input**: Returns an error for missing or invalid input parameters

## Examples

### Reading a text file

```typescript
const result = await readFile({
  path: "./data.txt"
});

if ("content" in result) {
  console.log(result.content);
}
```

### Handling errors

```typescript
const result = await readFile({
  path: "/path/to/nonexistent.txt"
});

if ("error" in result) {
  console.error(`Error: ${result.error}`);
  if (result.code === "ENOENT") {
    console.log("File does not exist");
  }
}
```

### Using in a workflow with error handling

```yaml
name: safe-read
type: router
config:
  routes:
    - when: output.error == null
      task:
        type: basic
        task:
          tool: echo
          input:
            message: "File content: {{ output.content }}"
    - when: output.code == "ENOENT"
      task:
        type: basic
        task:
          tool: echo
          input:
            message: "File not found, using default"
task:
  tool: readFile
  input:
    path: "{{ input.configPath }}"
```

## Testing

Run tests with:

```bash
bun test
```

## License

MIT