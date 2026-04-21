# TypeScript Best Practices Reference

This reference provides practical guidelines for TypeScript configuration, type strategies, and patterns for everyday development.

## Core TypeScript Configuration

### Recommended tsconfig.json Settings

```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "moduleResolution": "Bundler",
    "target": "ES2022",
    "lib": ["ES2022", "DOM", "DOM.Iterable"],
    "skipLibCheck": true,
    "esModuleInterop": true
  }
}
```

**Key Settings Explained:**

| Setting | Value | Purpose |
|---------|-------|---------|
| `strict` | `true` | Enables all strict type-checking options |
| `noUncheckedIndexedAccess` | `true` | Adds `undefined` to index signatures for safer array/object access |
| `exactOptionalPropertyTypes` | `true` | Distinguishes between `undefined` and missing properties |
| `moduleResolution` | `"Bundler"` | Optimized for modern bundlers (Vite, esbuild, etc.) |
| `target` | `"ES2022"` | Modern JavaScript features without unnecessary transpilation |
| `skipLibCheck` | `true` | Faster builds by skipping declaration file type checking |

## Type Inference vs Explicit Types

### Strategic Type Declaration Guidelines

**Be explicit with:**
- Function parameters
- Public API boundaries
- Complex objects and configurations
- Generic type parameters that can't be inferred

**Let TypeScript infer:**
- Return types (when straightforward)
- Local variables
- Internal logic
- Simple assignments

### Examples

```typescript
// GOOD: Explicit parameters, inferred return type
const processUser = (user: User) => ({
  id: user.id,
  fullName: `${user.firstName} ${user.lastName}`,
  isActive: user.lastLogin > Date.now() - 30 * 24 * 60 * 60 * 1000,
  membershipLevel: calculateMembershipLevel(user),
});

// GOOD: Complex configuration with explicit type
const apiConfig: ApiConfiguration = {
  baseUrl: process.env.API_URL,
  timeout: 5000,
  retries: 3,
  headers: {
    "Content-Type": "application/json",
  },
};

// BAD: Unnecessary explicit typing
const name: string = "John"; // Type is obvious
const count: number = items.length; // Type is obvious
```

## Interfaces vs Types

### When to Use Interfaces

- Object shapes that might be extended
- Public APIs
- Declaration merging is needed
- Better error messages preferred

### When to Use Type Aliases

- Union types
- Primitives and tuples
- Computed/mapped types
- Internal utilities and complex transformations

### Examples

```typescript
// Interface for extensible object shapes
interface User {
  id: string;
  name: string;
  email: string;
}

// Extending with intersection
type AdminUser = User & {
  permissions: Permission[];
  lastLogin: Date;
};

// Type for unions and computed types
type Status = "pending" | "approved" | "rejected";
type UserWithStatus = User & { status: Status };
type UserKeys = keyof User;
type OptionalUser = Partial<User>;

// Type for complex transformations
type ApiResponse<T> = {
  data: T;
  meta: {
    page: number;
    total: number;
  };
  errors?: string[];
};
```

## Module and Path Mapping

### tsconfig.json Path Configuration

```json
{
  "compilerOptions": {
    "baseUrl": ".",
    "paths": {
      "@/*": ["./src/*"],
      "@/components/*": ["./src/components/*"],
      "@/utils/*": ["./src/utils/*"],
      "@/types/*": ["./src/types/*"],
      "@/hooks/*": ["./src/hooks/*"]
    }
  }
}
```

### Barrel Exports

Create index files to group related exports:

```typescript
// utils/index.ts
export { formatDate, parseDate } from "./date";
export { validateEmail, validatePassword } from "./validation";
export { debounce, throttle } from "./performance";
```

### Import Patterns

```typescript
// GOOD: Clean imports with path aliases
import { Button, Card } from "@/components/ui";
import { useAuth } from "@/hooks/useAuth";
import { ApiClient } from "@/utils/api";
import type { User, Post } from "@/types/entities";

// GOOD: Relative imports for local files
import { PostCard } from "./PostCard";
import { usePostFilters } from "./usePostFilters";
```

## Error Handling Patterns

### Custom Error Classes

```typescript
class ValidationError extends Error {
  constructor(
    message: string,
    public field: string,
    public value: unknown
  ) {
    super(message);
    this.name = "ValidationError";
  }
}

class NotFoundError extends Error {
  constructor(resource: string, id: string) {
    super(`${resource} with id ${id} not found`);
    this.name = "NotFoundError";
  }
}
```

### Result Pattern for Functional Error Handling

```typescript
type Result<T, E = Error> =
  | { success: true; data: T }
  | { success: false; error: E };

async function safeApiCall<T>(apiCall: () => Promise<T>): Promise<Result<T>> {
  try {
    const data = await apiCall();
    return { success: true, data };
  } catch (error) {
    return {
      success: false,
      error: error instanceof Error ? error : new Error(String(error)),
    };
  }
}

// Usage
const result = await safeApiCall(() => fetchUser(userId));
if (result.success) {
  console.log(result.data.name); // TypeScript knows data exists
} else {
  console.error(result.error.message); // TypeScript knows error exists
}
```

## Performance Considerations

### Type System Performance Guidelines

1. **Avoid deeply nested conditional types** - They slow down the compiler
2. **Use type assertions sparingly and safely** - Prefer type guards
3. **Implement proper type guards** - For runtime safety with compile-time benefits
4. **Minimize complex union types in hot paths** - Keep unions small and simple

### Simple Type Guards

```typescript
function isString(value: unknown): value is string {
  return typeof value === "string";
}

function isUser(obj: unknown): obj is User {
  return (
    typeof obj === "object" &&
    obj !== null &&
    "id" in obj &&
    "name" in obj &&
    "email" in obj
  );
}
```

### Safe Type Assertions with Validation

```typescript
function assertIsNumber(value: unknown): asserts value is number {
  if (typeof value !== "number") {
    throw new Error(`Expected number, got ${typeof value}`);
  }
}
```

### Efficient vs Complex Types

```typescript
// GOOD: Efficient union types
type EventType = "click" | "hover" | "focus"; // Small, efficient union

// AVOID: Complex nested conditional types in hot paths
type ComplexConditional<T> = T extends string
  ? T extends `${infer U}Id`
    ? U extends "user"
      ? UserEvent
      : U extends "post"
        ? PostEvent
        : never
    : never
  : never;
```

## Type Testing Patterns

### Type-Level Testing Utilities

```typescript
// Assertion type - fails compilation if condition is false
type Expect<T extends true> = T;

// Equality check - returns true if types are exactly equal
type Equal<X, Y> = (<T>() => T extends X ? 1 : 2) extends <T>() => T extends Y ? 1 : 2
  ? true
  : false;

// Not equal check
type NotEqual<X, Y> = true extends Equal<X, Y> ? false : true;
```

### Testing Type Behavior

```typescript
// Test that a function returns the expected type
type TestGroupBy = Expect<Equal<ReturnType<typeof groupBy<User, "status">>, Record<string, User[]>>>;

// Test generic constraints at compile time
function testGenericConstraints() {
  const users: User[] = [];

  // Should work
  const groupedByStatus = groupBy(users, "status");

  // Should fail type checking (uncomment to verify)
  // const groupedByInvalid = groupBy(users, 'invalidKey')
}
```

### Runtime Type Testing with Vitest/Jest

```typescript
describe("Type Guards", () => {
  test("isUser correctly identifies User objects", () => {
    const validUser = { id: "1", name: "John", email: "john@example.com" };
    const invalidUser = { id: "1", name: "John" }; // missing email

    expect(isUser(validUser)).toBe(true);
    expect(isUser(invalidUser)).toBe(false);
    expect(isUser(null)).toBe(false);
    expect(isUser("string")).toBe(false);
  });
});
```

## Branded Types for Type Safety

Use branded types to prevent mixing semantically different values of the same base type:

```typescript
type UserId = string & { readonly brand: unique symbol };
type PostId = string & { readonly brand: unique symbol };

function createUserId(id: string): UserId {
  return id as UserId;
}

function getUser(id: UserId) {
  // This function can only accept UserId, not any string or PostId
  return users.find((user) => user.id === id);
}

// Usage
const userId = createUserId("user-123");
const postId = createPostId("post-456");

getUser(userId); // OK
// getUser(postId); // Error: Argument of type 'PostId' is not assignable to parameter of type 'UserId'
```

## Quick Reference Summary

| Category | Guideline |
|----------|-----------|
| Configuration | Enable `strict`, `noUncheckedIndexedAccess`, `exactOptionalPropertyTypes` |
| Type Inference | Explicit at boundaries, inferred internally |
| Interfaces vs Types | Interfaces for objects, types for unions/computed |
| Imports | Use path aliases for cross-feature, relative for local |
| Error Handling | Use Result pattern or custom error classes |
| Performance | Keep types simple, avoid deep nesting |
| Testing | Use `Expect<Equal<X, Y>>` for type-level tests |
