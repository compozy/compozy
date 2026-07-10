---
name: typescript-advanced
description: Master TypeScript's advanced type system including generics, conditional types, mapped types, template literals, and utility types for building type-safe applications. Use when implementing complex type logic, creating reusable type utilities, or ensuring compile-time type safety in TypeScript projects. Don't use for plain JavaScript, runtime validation libraries (Zod, Yup), or basic TypeScript syntax questions.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# TypeScript Advanced Types

Comprehensive guidance for mastering TypeScript's advanced type system. This SKILL.md is a **dispatcher** — the detailed code patterns live in `references/*.md` and are loaded on demand.

## Required Reading Router

Read the matching reference before producing code. Each is one level deep and self-contained.

| Goal | Read |
|------|------|
| Refresh foundational generics, conditional types, mapped types, template literals, utility types | `references/core-concepts.md` |
| Build a type-safe event emitter, API client, builder, deep utility, form validator, or discriminated state machine | `references/advanced-patterns.md` |
| Use `infer`, type guards, assertion functions, or write compile-time type tests | `references/type-inference.md` |
| Apply project-level configuration, interfaces-vs-types, error handling, performance | `references/best-practices.md` |

## When to Use This Skill

- Building type-safe libraries or frameworks
- Creating reusable generic components
- Implementing complex type inference logic
- Designing type-safe API clients
- Building form validation systems
- Creating strongly-typed configuration objects
- Implementing type-safe state management
- Migrating JavaScript codebases to TypeScript

## Decision Map

Use these decision points to pick the right tool before writing code:

| Scenario | Recommendation |
|----------|----------------|
| Function parameters | Always explicit |
| Return types | Let TypeScript infer (usually) |
| Local variables | Let TypeScript infer |
| Public API boundaries | Always explicit |
| Object shapes (extensible) | Use `interface` |
| Union types | Use `type` |
| Computed/mapped types | Use `type` |
| Runtime-narrowing of `unknown` | Type guard or assertion (see `references/type-inference.md`) |
| Variant data with shared field | Discriminated union (see `references/advanced-patterns.md` Pattern 6) |

## Core Guidelines

1. **Use `unknown` over `any`** — Enforce type checking at boundaries.
2. **Prefer `interface` for object shapes** — Better error messages, declaration merging.
3. **Use `type` for unions and computed types** — More flexible for mapped/conditional types.
4. **Leverage type inference** — Let TypeScript infer when possible; annotate only at boundaries.
5. **Create helper types** — Build reusable type utilities (see `references/advanced-patterns.md`).
6. **Use const assertions** — Preserve literal types.
7. **Avoid type assertions** — Prefer type guards (see `references/type-inference.md`).
8. **Document complex types** — Add JSDoc on exported public types.
9. **Use strict mode** — Enable all strict compiler options.
10. **Test your types** — Use `Expect<Equal<X, Y>>` compile-time tests (see `references/type-inference.md`).

## Recommended `tsconfig.json`

```json
{
  "compilerOptions": {
    "strict": true,
    "noUncheckedIndexedAccess": true,
    "exactOptionalPropertyTypes": true,
    "moduleResolution": "Bundler"
  }
}
```

For deeper configuration rationale, read `references/best-practices.md`.

## Common Pitfalls

1. **Over-using `any`** — Defeats the purpose of TypeScript.
2. **Ignoring strict null checks** — Can lead to runtime errors.
3. **Too complex types** — Can slow down compilation.
4. **Not using discriminated unions** — Misses type narrowing opportunities.
5. **Forgetting `readonly` modifiers** — Allows unintended mutations.
6. **Circular type references** — Can cause compiler errors.
7. **Not handling edge cases** — Empty arrays, null values, missing keys.

## Performance Considerations

- Avoid deeply nested conditional types.
- Use simple types when possible.
- Cache complex type computations behind `type Alias = ...`.
- Limit recursion depth in recursive types (TS caps around 50).
- Use build tools to skip type checking in production.
- Prefer type guards over type assertions for runtime safety.
- Keep union types small in hot paths.

See `references/best-practices.md#performance-considerations` for deeper analysis.

## External Resources

- TypeScript Handbook — https://www.typescriptlang.org/docs/handbook/
- Type Challenges — https://github.com/type-challenges/type-challenges
- TypeScript Deep Dive — https://basarat.gitbook.io/typescript/
- Effective TypeScript — book by Dan Vanderkam
