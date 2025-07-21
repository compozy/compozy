# Tool Config Parameter Example

This example demonstrates the new config parameter feature for tools in Compozy.

## Overview

Tools can now receive two separate parameters:
1. **Input**: Runtime data that changes with each invocation
2. **Config**: Static configuration that remains constant

This separation allows tools to have default settings, API endpoints, formatting preferences, and other configuration that doesn't need to be passed with every call.

## Example Tools

### API Caller Tool

Demonstrates how to use config for:
- Base URL configuration
- Default timeout settings
- Retry count
- Custom headers

### Formatter Tool

Shows config usage for:
- Default output format
- Indentation settings
- Sorting preferences
- Date formatting
- Number precision

## Running the Example

```bash
# From the project root
compozy run examples/tool-config/workflow.yaml
```

## Config Parameter Benefits

1. **Separation of Concerns**: Keep static configuration separate from runtime input
2. **Reusability**: Tools can have sensible defaults without hardcoding
3. **Flexibility**: Override config values when needed
4. **Type Safety**: Config can be validated with JSON schema
5. **Environment-Specific Settings**: Different configs for dev/staging/prod

## Implementation Details

In TypeScript tools, the config parameter is passed as the second argument:

```typescript
export async function my_tool(input: InputType, config?: ConfigType) {
  // Use config values with defaults
  const setting = config?.setting || "default";
  // ... tool implementation
}
```

In YAML workflow definitions, config is specified at the tool level:

```yaml
tools:
  - id: my-tool
    config:
      setting: "custom-value"
      timeout: 30
```