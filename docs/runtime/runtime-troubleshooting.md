# Runtime Troubleshooting Guide

This guide covers common issues and solutions when working with Compozy's runtime system. Use this as a reference for debugging runtime-related problems.

## Common Issues

### 1. Bun Not Found

**Error Messages:**

```
bun executable not found in PATH
failed to create worker: bun not available
```

**Solutions:**

1. **Install Bun**

    ```bash
    # macOS/Linux
    curl -fsSL https://bun.sh/install | bash
    
    # Windows
    powershell -c "irm bun.sh/install.ps1 | iex"
    ```

2. **Verify Installation**

    ```bash
    bun --version
    which bun
    ```

3. **Add to PATH (if needed)**
    ```bash
    echo 'export PATH="$HOME/.bun/bin:$PATH"' >> ~/.bashrc
    source ~/.bashrc
    ```

### 2. Worker File Not Found

**Error Messages:**

```
worker file not found at .compozy/bun_worker.ts
run 'compozy dev' to generate it
```

**Solutions:**

1. **Generate Worker Files**

    ```bash
    compozy dev
    # or explicitly
    compozy worker generate
    ```

2. **Check Directory Structure**

    ```bash
    ls -la .compozy/
    # Should contain: bun_worker.ts
    ```

3. **Verify Permissions**
    ```bash
    chmod 644 .compozy/bun_worker.ts
    ```

### 3. Tool Not Found in Entrypoint

**Error Messages:**

```
Tool weather_tool not found in entrypoint exports
Available tools: none
```

**Solutions:**

1. **Check Entrypoint File**

    ```typescript
    // entrypoint.ts
    export { weather_tool } from "./tools/weather_tool.ts";
    ```

2. **Verify Function Export**

    ```typescript
    // tools/weather_tool.ts
    export function weather_tool(input: any) {
        // Implementation
    }
    ```

3. **Check File Paths**
    - Ensure relative paths are correct
    - Use explicit `.ts` extensions
    - Verify files exist and are readable

4. **Debug Entrypoint**
    ```typescript
    // entrypoint.ts - Add debugging
    console.error("Available exports:", Object.keys(import.meta));
    export { weather_tool } from "./tools/weather_tool.ts";
    ```

### 4. Permission Denied Errors

**Error Messages:**

```
Error: Access denied
Tool execution failed: permission error
```

**Solutions:**

1. **Check Runtime Permissions**

    ```yaml
    # compozy.yaml
    runtime:
        type: bun
        permissions:
            - --allow-read # File system read
            - --allow-write # File system write
            - --allow-net # Network access
            - --allow-env # Environment variables
    ```

2. **Common Permission Sets**

    ```yaml
    # Minimal (read-only tools)
    permissions:
      - --allow-read

    # Standard (most tools)
    permissions:
      - --allow-read
      - --allow-net

    # Full access (if needed)
    permissions:
      - --allow-read
      - --allow-write
      - --allow-net
      - --allow-env
    ```

3. **Verify File System Permissions**
    ```bash
    # Check project directory permissions
    ls -la .
    chmod -R 755 tools/
    ```

### 5. Import Resolution Issues

**Error Messages:**

```
Cannot resolve module './tools/weather_tool.ts'
Module not found
```

**Solutions:**

1. **Use Explicit Extensions**

    ```typescript
    // Correct
    export { weather_tool } from "./tools/weather_tool.ts";

    // Incorrect
    export { weather_tool } from "./tools/weather_tool";
    ```

2. **Check Relative Paths**

    ```typescript
    // From entrypoint.ts in project root
    export { tool } from "./tools/tool.ts"; // ✓ Correct
    export { tool } from "./subdirectory/tool.ts"; // ✓ Correct
    export { tool } from "tools/tool.ts"; // ✗ Incorrect
    ```

3. **Verify File Structure**
    ```
    project/
    ├── entrypoint.ts
    └── tools/
        └── weather_tool.ts
    ```

### 6. TypeScript Compilation Errors

**Error Messages:**

```
Type error in entrypoint.ts
Cannot find module or its corresponding type declarations
```

**Solutions:**

1. **Add Type Declarations**

    ```typescript
    // types/tool.ts
    export interface ToolFunction<TInput = any, TOutput = any> {
        (input: TInput): Promise<TOutput> | TOutput;
    }
    ```

2. **Configure TypeScript**

    ```json
    // tsconfig.json
    {
        "compilerOptions": {
            "target": "ES2022",
            "module": "ESNext",
            "moduleResolution": "node",
            "esModuleInterop": true,
            "allowSyntheticDefaultImports": true,
            "strict": false,
            "skipLibCheck": true
        }
    }
    ```

3. **Use Type Assertions (temporary)**
    ```typescript
    // Temporary workaround
    export { weather_tool } from "./tools/weather_tool.ts" as any;
    ```

### 7. Tool Execution Timeouts

**Error Messages:**

```
Tool execution timed out after 60000ms
Context deadline exceeded
```

**Solutions:**

1. **Increase Global Timeout**

    ```yaml
    # compozy.yaml
    runtime:
        type: bun
        timeout: 120s # Increase from default 60s
    ```

2. **Optimize Tool Code**

    ```typescript
    // Add progress logging
    export async function weather_tool(input: any) {
        console.log("Starting weather fetch...");
        const data = await fetch(url);
        console.log("Fetch completed");
        return data;
    }
    ```

3. **Use Streaming for Large Operations**
    ```typescript
    // For large data processing
    export async function process_tool(input: any) {
        // Process in chunks
        for (const chunk of chunks) {
            await processChunk(chunk);
            // Allow other operations to run
            await new Promise((resolve) => setTimeout(resolve, 0));
        }
    }
    ```

### 8. Environment Variable Issues

**Error Messages:**

```
Environment variable API_KEY is not allowed for security reasons
Invalid environment variable name: api_key
```

**Solutions:**

1. **Use Proper Variable Names**

    ```typescript
    // Correct - uppercase with underscores
    process.env.API_KEY;
    process.env.DATABASE_URL;

    // Incorrect - lowercase or special chars
    process.env.api_key; // ✗
    process.env.api - key; // ✗
    ```

2. **Check Blocked Variables**

    ```typescript
    // These are blocked for security:
    // LD_PRELOAD, DYLD_INSERT_LIBRARIES, NODE_OPTIONS, etc.

    // Use project-specific variables instead
    process.env.COMPOZY_API_KEY; // ✓
    process.env.TOOL_CONFIG; // ✓
    ```

3. **Validate Variable Values**
    ```typescript
    // Avoid newlines and null bytes
    const apiKey = process.env.API_KEY?.replace(/[\n\r\0]/g, "");
    ```

### 9. Memory and Performance Issues

**Error Messages:**

```
JavaScript heap out of memory
Tool execution failed: resource exhaustion
```

**Solutions:**

1. **Monitor Memory Usage**

    ```typescript
    // Add memory monitoring
    export async function large_data_tool(input: any) {
        console.log("Memory usage:", process.memoryUsage());
        // Tool implementation
        console.log("Memory after:", process.memoryUsage());
    }
    ```

2. **Process Data in Chunks**

    ```typescript
    // Instead of loading all data at once
    const allData = await loadLargeDataset(); // ✗

    // Process in chunks
    for await (const chunk of loadDatasetChunks()) {
        // ✓
        await processChunk(chunk);
    }
    ```

3. **Clean Up Resources**
    ```typescript
    export async function file_tool(input: any) {
        let file;
        try {
            file = await Bun.file(input.path);
            return await processFile(file);
        } finally {
            // Clean up if needed
            file = null;
        }
    }
    ```

### 10. JSON Parsing Errors

**Error Messages:**

```
Failed to parse JSON input
Invalid JSON in tool response
```

**Solutions:**

1. **Validate Input**

    ```typescript
    export function tool(input: any) {
        if (!input || typeof input !== "object") {
            throw new Error("Input must be a valid object");
        }
        // Implementation
    }
    ```

2. **Handle Large Outputs**

    ```typescript
    export function tool(input: any) {
        const result = processData(input);

        // Check output size
        const jsonString = JSON.stringify(result);
        if (jsonString.length > 10 * 1024 * 1024) {
            // 10MB
            return { error: "Output too large, use streaming instead" };
        }

        return result;
    }
    ```

3. **Sanitize Output**

    ```typescript
    export function tool(input: any) {
        const result = processData(input);

        // Remove circular references
        return JSON.parse(JSON.stringify(result));
    }
    ```

## Debugging Techniques

### 1. Enable Debug Logging

```typescript
// entrypoint.ts
console.error("Loading tools...");
export { weather_tool } from "./tools/weather_tool.ts";
console.error("Tools loaded successfully");
```

### 2. Test Tools Independently

```typescript
// test-tool.ts
import { weather_tool } from "./tools/weather_tool.ts";

async function test() {
    try {
        const result = await weather_tool({ city: "London" });
        console.log("Success:", result);
    } catch (error) {
        console.error("Error:", error);
    }
}

test();
```

### 3. Check Runtime Worker

```bash
# Test worker directly
echo '{"tool_id":"weather_tool","tool_exec_id":"test","input":{"city":"London"},"env":{}}' \
    | bun .compozy/bun_worker.ts
```

### 4. Validate Configuration

```bash
# Check YAML syntax
compozy config validate

# Check tool discovery
compozy tools list
```

## Getting Help

### 1. Enable Verbose Logging

```bash
export COMPOZY_LOG_LEVEL=debug
compozy run workflow
```

### 2. Check System Information

```bash
# Check Bun version
bun --version

# Check Node version (if using)
node --version

# Check project structure
find . -name "*.ts" -o -name "*.yaml" | head -20
```

### 3. Community Resources

- **Documentation**: [Compozy Docs](https://docs.compozy.ai)
- **GitHub Issues**: [Report Issues](https://github.com/compozy/compozy/issues)
- **Discord Community**: [Join Discussion](https://discord.gg/compozy)
- **Example Projects**: Check the `examples/` directory

### 4. Creating Minimal Reproductions

When reporting issues, create a minimal reproduction:

```typescript
// minimal-entrypoint.ts
export function simple_tool(input: { message: string }) {
    return { result: input.message };
}
```

```yaml
# minimal-compozy.yaml
name: minimal-test
version: 0.1.0

runtime:
    type: bun
    entrypoint: "./minimal-entrypoint.ts"
    permissions:
        - --allow-read
```

This helps identify whether issues are with your specific configuration or the runtime system itself.
