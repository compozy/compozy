# Detection Catalog

Classification reference for `architectural-analysis`. Each SKILL step names the section to load when classifying that dimension's findings.

## Dead code

Categories:
- **Dead** — exported but never imported; function never called; class instantiated nowhere; type never used; constant never referenced; file with no inbound imports.
- **Possibly dead** (needs verification) — used only in commented-out code, in other dead code, in other unused exports, or in tests for a deprecated feature.
- **Internal dead code** — non-exported function never called; variable assigned but never read; parameter accepted but never used.

Not dead — treat as USED even with no local usage:
- used in tests (may be public API)
- dynamically imported/required
- referenced via reflection or string keys
- part of the public API even if unused internally
- framework hooks (lifecycle methods, callbacks)
- accessed via `window` or global scope

Confidence: **HIGH** (no usage anywhere), **MEDIUM** (only indirect or uncertain usage), **LOW** (plausible dynamic/reflective use).

## Duplication

Confirm by reading the implementations — same logic, same cases handled, one could replace the other — not by matching names. Classify and rank:
- **Exact (CRITICAL)** — identical or near-identical code, copy-pasted functions or utilities. A bug fix means editing every copy.
- **Similar logic (HIGH)** — same algorithm, different implementation, params, or name. Inconsistency risk.
- **Conceptual (MEDIUM)** — competing ways to do one thing; overlapping utilities.
- **Type (HIGH)** — same interface/type/enum/constant defined in several places, or types that should share a base.

## Anti-patterns
- **God object** — file over 500 lines, class with 10+ methods, or one module holding many responsibilities and importing from everywhere.
- **Circular dependency** — A↔B, or chains A→B→C→A.
- **Tight coupling** — high-level modules depending on low-level ones; business logic on infrastructure; core logic on framework specifics.
- **Layer violation** — components importing the database layer, models importing views, utilities importing business logic.
- **Singleton abuse** — global or module-level mutable state; static methods over shared state.
- **Anemic domain model** — data classes with only getters/setters, all behavior pushed into services.
- **Shotgun surgery** — one feature change forces edits across many files (poor cohesion).
- **Feature envy** — a method using more of another class's data than its own.

## Type issues
- **`any` / implicit `any`** — decide whether a proper type can be defined; flag missing annotations that fall back to `any`.
- **Unsafe assertions** — `as any`, `as unknown as T`, or casts without validation; may hide a real type error.
- **`@ts-ignore` / `@ts-expect-error`** — may mask an actual problem; refactor rather than suppress.
- **Type duplication** — same interface across files (cross-reference Duplication → Type).
- **Missing types** — functions without return types, untyped callbacks, over-broad generics.

## Code smells
- **Long function** — over 50 lines; likely doing too much.
- **Long parameter list** — 4+ params; prefer a grouped object.
- **Complex conditional** — nesting 3+ deep, boolean expressions spanning lines, or a switch with 10+ cases.
- **Magic number/string** — unexplained literals or repeated string literals; name them as constants.
- **Commented-out code** — delete it; git history preserves it.
- **Poor naming** — single-letter names outside loops, context-free abbreviations (`usr`, `msg`, `tmp`), or misleading names.
