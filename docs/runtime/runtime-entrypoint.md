# Runtime Entrypoint Pattern Guide

The entrypoint pattern is the new standard for organizing and exposing tools in Compozy's runtime system. This guide covers how to create, structure, and manage entrypoint files for optimal tool execution.

## Overview

The entrypoint pattern replaces the old Deno-based execution model with a more flexible and secure approach:

- **Single Entry Point**: All tools are imported and exported from one entrypoint file
- **Named Exports**: Tools are exposed as named function exports
- **Type Safety**: Full TypeScript support with proper type definitions
- **Security**: Built-in validation and sandboxing through the runtime system

## Basic Entrypoint Structure

### Minimal Entrypoint File

Create an `entrypoint.ts` file in your project root:

```typescript
// entrypoint.ts
import { weatherTool } from "./tools/weather_tool.ts";
import { saveTool } from "./tools/save_tool.ts";

// Export tools with snake_case keys for Compozy runtime
export const weather_tool = weatherTool;
export const save_tool = saveTool;
```

### Tool File Structure

Each tool file exports a function using camelCase naming:

```typescript
// tools/weather_tool.ts
interface WeatherInput {
    city: string;
}

interface WeatherOutput {
    temperature: number;
    humidity: number;
    weather: string;
}

export async function weatherTool(input: WeatherInput): Promise<WeatherOutput> {
    // Tool implementation
    return {
        temperature: 22,
        humidity: 65,
        weather: "Partly cloudy",
    };
}
```

## Advanced Patterns

### Conditional Tool Loading

```typescript
// entrypoint.ts
import { commonTool } from "./tools/common_tool.ts";

// Base exports
export const common_tool = commonTool;

// Conditional exports based on environment
if (process.env.NODE_ENV === "development") {
    const { debugTool } = await import("./tools/debug_tool.ts");
    export const debug_tool = debugTool;
}

// Feature flag exports
if (process.env.FEATURE_WEATHER === "true") {
    const { weatherTool } = await import("./tools/weather_tool.ts");
    export const weather_tool = weatherTool;
}
```

### Dynamic Tool Discovery

```typescript
// entrypoint.ts
import { readdirSync } from "fs";
import { join } from "path";

// Auto-discover and export all tools from tools directory
const toolsDir = "./tools";
const toolFiles = readdirSync(toolsDir)
    .filter(file => file.endsWith("_tool.ts"))
    .filter(file => !file.startsWith("_")); // Skip private tools

for (const file of toolFiles) {
    const toolName = file.replace(".ts", "");
    const toolModule = await import(join(toolsDir, file));

    // Export the main function from each tool
    if (typeof toolModule[toolName] === "function") {
        export { [toolName]: toolModule[toolName] };
    }
}
```

### Tool Composition and Middleware

```typescript
// entrypoint.ts
import { withLogging, withValidation, withTimeout } from "./middleware";

// Basic tool import
import { weather_tool as _weather_tool } from "./tools/weather_tool.ts";

// Enhanced tool with middleware
export const weather_tool = withLogging(
    withValidation(
        withTimeout(_weather_tool, 30000), // 30 second timeout
    ),
);
```

### Middleware Example

```typescript
// middleware.ts
export function withLogging<T extends (...args: any[]) => any>(fn: T): T {
    return ((...args: any[]) => {
        console.log(`Executing ${fn.name} with args:`, args);
        const result = fn(...args);
        console.log(`Result:`, result);
        return result;
    }) as T;
}

export function withValidation<T extends (input: any) => any>(fn: T): T {
    return ((input: any) => {
        if (!input || typeof input !== "object") {
            throw new Error("Invalid input: must be an object");
        }
        return fn(input);
    }) as T;
}

export function withTimeout<T extends (...args: any[]) => Promise<any>>(
    fn: T,
    timeoutMs: number,
): T {
    return (async (...args: any[]) => {
        return Promise.race([
            fn(...args),
            new Promise((_, reject) =>
                setTimeout(() => reject(new Error(`Timeout after ${timeoutMs}ms`)), timeoutMs),
            ),
        ]);
    }) as T;
}
```

## Error Handling Best Practices

### Structured Error Responses

```typescript
// tools/api_tool.ts
interface ApiError {
    code: string;
    message: string;
    details?: any;
}

interface ApiResponse<T> {
    success: boolean;
    data?: T;
    error?: ApiError;
}

export async function api_tool(input: { url: string }): Promise<ApiResponse<any>> {
    try {
        const response = await fetch(input.url);
        const data = await response.json();

        return {
            success: true,
            data,
        };
    } catch (error) {
        return {
            success: false,
            error: {
                code: "FETCH_ERROR",
                message: error instanceof Error ? error.message : "Unknown error",
                details: { url: input.url },
            },
        };
    }
}
```

### Error Boundary Pattern

```typescript
// entrypoint.ts
function withErrorBoundary<T extends (...args: any[]) => any>(fn: T): T {
    return ((...args: any[]) => {
        try {
            const result = fn(...args);

            // Handle async functions
            if (result instanceof Promise) {
                return result.catch((error) => ({
                    success: false,
                    error: {
                        message: error.message,
                        stack: error.stack,
                        timestamp: new Date().toISOString(),
                    },
                }));
            }

            return result;
        } catch (error) {
            return {
                success: false,
                error: {
                    message: error instanceof Error ? error.message : "Unknown error",
                    timestamp: new Date().toISOString(),
                },
            };
        }
    }) as T;
}

// Apply error boundary to all tools
import { weather_tool as _weather_tool } from "./tools/weather_tool.ts";
export const weather_tool = withErrorBoundary(_weather_tool);
```

## TypeScript Configuration

### Recommended `tsconfig.json`

```json
{
    "compilerOptions": {
        "target": "ES2022",
        "module": "ESNext",
        "moduleResolution": "node",
        "esModuleInterop": true,
        "allowSyntheticDefaultImports": true,
        "strict": true,
        "skipLibCheck": true,
        "forceConsistentCasingInFileNames": true,
        "declaration": true,
        "outDir": "./dist",
        "rootDir": "./",
        "types": ["bun-types"]
    },
    "include": ["entrypoint.ts", "tools/**/*", "middleware/**/*", "types/**/*"],
    "exclude": ["node_modules", "dist", ".compozy"]
}
```

### Type Definitions

```typescript
// types/tool.ts
export interface ToolInput {
    [key: string]: any;
}

export interface ToolOutput {
    [key: string]: any;
}

export interface ToolFunction<TInput = ToolInput, TOutput = ToolOutput> {
    (input: TInput): Promise<TOutput> | TOutput;
}

export interface ToolMetadata {
    name: string;
    description: string;
    version: string;
    schema?: {
        input: any;
        output: any;
    };
}
```

## File Organization Best Practices

### Recommended Project Structure

```
project/
├── entrypoint.ts              # Main entrypoint file
├── tools/                     # Tool implementations
│   ├── weather_tool.ts
│   ├── save_tool.ts
│   └── api_tool.ts
├── middleware/                # Shared middleware
│   ├── logging.ts
│   ├── validation.ts
│   └── timeout.ts
├── types/                     # Type definitions
│   ├── tool.ts
│   └── common.ts
├── utils/                     # Shared utilities
│   ├── http.ts
│   └── validation.ts
└── tests/                     # Test files
    ├── weather_tool.test.ts
    └── integration.test.ts
```

### Naming Conventions

- **Tool Files**: Use `snake_case` with `_tool` suffix (e.g., `weather_tool.ts`)
- **Export Names**: Match the filename without extension (e.g., `weather_tool`)
- **Function Names**: Use descriptive names that indicate the tool's purpose
- **Directories**: Use `kebab-case` for multi-word directories

### Import Patterns

```typescript
// entrypoint.ts

// Preferred: Default export object with snake_case keys (New Standard)
import { weatherTool } from "./tools/weather_tool.ts";
import { saveTool } from "./tools/save_tool.ts";

export default {
    weather_tool: weatherTool,
    save_tool: saveTool,
};

// Legacy: Named exports (still supported for backwards compatibility)
export const weather_tool = weatherTool;
export const save_tool = saveTool;

// Alternative: Namespace imports for organization
import * as WeatherTools from "./tools/weather";
import * as FileTools from "./tools/files";

export const { current_weather: weather_tool, forecast: weather_forecast_tool } = WeatherTools;

export const { save_file: save_tool, read_file: read_tool } = FileTools;
```

## Security Considerations

### Input Validation

```typescript
// tools/secure_tool.ts
import { z } from "zod";

const InputSchema = z.object({
    userId: z.string().uuid(),
    action: z.enum(["read", "write", "delete"]),
    data: z.string().max(1024), // Limit data size
});

export async function secure_tool(input: unknown) {
    // Validate input
    const validInput = InputSchema.parse(input);

    // Additional security checks
    if (validInput.action === "delete" && !validInput.userId) {
        throw new Error("Delete operations require valid user ID");
    }

    // Implementation...
}
```

### Environment Variable Handling

```typescript
// tools/env_tool.ts
export async function env_tool(input: { key: string }) {
    // Whitelist allowed environment variables
    const allowedVars = ["NODE_ENV", "API_BASE_URL", "CUSTOM_CONFIG"];

    if (!allowedVars.includes(input.key)) {
        throw new Error(`Environment variable ${input.key} is not allowed`);
    }

    return {
        key: input.key,
        value: process.env[input.key] || null,
    };
}
```

## Testing Entrypoint Files

### Unit Testing Tools

```typescript
// tests/weather_tool.test.ts
import { weather_tool } from "../tools/weather_tool.ts";

describe("weather_tool", () => {
    test("should return weather data for valid city", async () => {
        const result = await weather_tool({ city: "London" });

        expect(result).toHaveProperty("temperature");
        expect(result).toHaveProperty("humidity");
        expect(result).toHaveProperty("weather");
        expect(typeof result.temperature).toBe("number");
    });

    test("should handle invalid city gracefully", async () => {
        const result = await weather_tool({ city: "InvalidCity123" });

        expect(result).toEqual({
            temperature: 20,
            humidity: 50,
            weather: "Clear sky",
        });
    });
});
```

### Integration Testing

```typescript
// tests/entrypoint.test.ts
import * as entrypoint from "../entrypoint.ts";

describe("entrypoint", () => {
    test("should export all required tools", () => {
        expect(typeof entrypoint.weather_tool).toBe("function");
        expect(typeof entrypoint.save_tool).toBe("function");
    });

    test("tools should be callable", async () => {
        const tools = Object.entries(entrypoint).filter(
            ([_, value]) => typeof value === "function",
        );

        expect(tools.length).toBeGreaterThan(0);

        for (const [name, tool] of tools) {
            expect(tool).toBeInstanceOf(Function);
        }
    });
});
```

## Migration from Legacy Pattern

### Before (Deno + execute property)

```yaml
# Old compozy.yaml
tools:
    - id: weather_tool
      execute: weather_tool.ts
```

```typescript
// Old weather_tool.ts
export async function run(input: { city: string }) {
    // Implementation
}
```

### After (Entrypoint pattern)

```typescript
// New entrypoint.ts
import { weatherTool } from "./tools/weather_tool.ts";

export const weather_tool = weatherTool;
```

```typescript
// New tools/weather_tool.ts
export async function weatherTool(input: { city: string }) {
    // Same implementation, function name now follows camelCase
}
```

```yaml
# New compozy.yaml (remove execute property)
tools:
    - id: weather_tool
      # execute property removed
```

## Common Issues and Solutions

### Tool Not Found Error

**Problem**: `Tool weather_tool not found in entrypoint exports`

**Solutions**:

1. Verify the tool is exported in `entrypoint.ts`
2. Check that function names match between file and export
3. Ensure there are no syntax errors in the entrypoint file

### Import Resolution Issues

**Problem**: `Cannot resolve module './tools/weather_tool.ts'`

**Solutions**:

1. Use explicit file extensions in imports
2. Check file paths are relative to entrypoint location
3. Verify files exist and are readable

### Type Errors

**Problem**: TypeScript compilation errors

**Solutions**:

1. Add proper type definitions for all tool functions
2. Use `// @ts-ignore` sparingly for third-party modules
3. Configure `tsconfig.json` with appropriate settings

For more troubleshooting information, see [Runtime Troubleshooting Guide](runtime-troubleshooting.md).
