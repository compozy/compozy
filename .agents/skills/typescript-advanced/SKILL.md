---
name: typescript-advanced
description: "Implement advanced TypeScript types, reusable type utilities, and compile-time contracts with generics, conditional/mapped types, and template literals. Excludes basic syntax, JavaScript, and runtime validation."
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---
# TypeScript Advanced Types

Use this skill when generic inference, conditional/mapped types, template literals, or a reusable type utility is the actual problem. Ordinary typed API, form, and state work uses its domain guidance unless advanced type behavior needs attention.

| Concern being changed | Reference |
| --- | --- |
| Generics, conditional/mapped/template-literal and utility types | `references/core-concepts.md` |
| Reusable emitters, builders, clients, deep utilities, or discriminated models | `references/advanced-patterns.md` |
| `infer`, narrowing, assertion functions, and type-contract tests | `references/type-inference.md` |
| Compiler configuration, type complexity, and maintainability | `references/best-practices.md` |

Prefer the simplest type that expresses the contract. Use `unknown` and real narrowing at untrusted boundaries; preserve discriminants and readonly intent. Let inference handle local values and follow repository conventions for public annotations, `interface`, and `type`.

Use the existing `tsconfig`; reference configurations are examples, not instructions to change it. Add compile-time tests only when type behavior is a product/library contract, using its owning suite. Avoid a new type framework or elaborate generic abstraction for a single ordinary call site. Run the repository's affected typecheck gate.
