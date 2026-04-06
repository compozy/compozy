# Code Smells Catalog

Complete catalog based on Martin Fowler's "Refactoring" (2nd Edition, Chapter 3),
supplemented by refactoring.guru taxonomy and modern TypeScript/React patterns.

---

## Bloaters

Smells where code grows too large to work with effectively.

### Long Function
- **Heuristic**: >15 lines of logic, multiple levels of abstraction, or requires
  comments to explain sections.
- **Why it matters**: Short functions with good names are self-documenting. Fowler:
  "Classes with short methods live longest."
- **Fix**: Extract Function, Replace Temp with Query, Introduce Parameter Object,
  Decompose Conditional, Split Loop, Replace Loop with Pipeline.
- **Modern variant**: React components with >5 lines before `return` — extract to
  custom hooks.

### Large Class / Large Module
- **Heuristic**: >300 lines, many fields/exports, multiple unrelated responsibilities.
- **Why it matters**: Violates Single Responsibility. Hard to understand, test, and change.
- **Fix**: Extract Class, Extract Module, Extract Superclass, Replace Type Code with
  Subclasses.
- **Modern variant**: God components in React, barrel files that re-export everything.

### Long Parameter List
- **Heuristic**: >3 parameters in a function signature.
- **Why it matters**: Hard to call correctly, easy to swap arguments.
- **Fix**: Replace Parameter with Query, Preserve Whole Object, Introduce Parameter
  Object, Remove Flag Argument, Combine Functions into Class.
- **Modern variant**: React components with >5 props — consider composition or context.

### Data Clumps
- **Heuristic**: Same 3+ parameters or fields appear together repeatedly
  (e.g., `startDate`/`endDate`, `x`/`y`/`z`, `host`/`port`/`protocol`).
- **Why it matters**: Signals a missing abstraction.
- **Fix**: Extract Class, Introduce Parameter Object, Preserve Whole Object.

### Primitive Obsession
- **Heuristic**: Raw strings/numbers for domain concepts — money as `number`, email
  as `string`, status as string literal instead of enum.
- **Why it matters**: No validation, no behavior, easy to mix up (passing orderId
  where userId expected).
- **Fix**: Replace Primitive with Object, Replace Type Code with Subclasses.
- **Modern variant (TypeScript)**: Use branded types, discriminated unions, or
  Zod schemas instead of raw primitives.
  ```typescript
  // Smell
  function getUser(id: string): User
  // Better
  type UserId = string & { __brand: 'UserId' }
  function getUser(id: UserId): User
  ```

---

## Change Preventers

Smells that make changes expensive — touching one thing forces changes elsewhere.

### Divergent Change
- **Heuristic**: One module is modified for multiple unrelated reasons (e.g., a file
  changes for both database schema updates AND UI rendering changes).
- **Why it matters**: Violates SRP. Merge conflicts, unclear ownership.
- **Fix**: Split Phase, Move Function, Extract Function, Extract Class.

### Shotgun Surgery
- **Heuristic**: A single logical change requires edits in 5+ files scattered across
  the codebase.
- **Why it matters**: High risk of missing a spot, expensive to change.
- **Fix**: Move Function, Move Field, Combine Functions into Class, Combine Functions
  into Transform, Inline Function, Inline Class.

---

## Dispensables

Code that could be removed to make the codebase cleaner.

### Duplicated Code
- **Heuristic**: Same or near-identical code blocks in 2+ locations. Apply the
  Rule of Three — "The first time, just do it. The second time, wince. The third
  time, refactor."
- **Why it matters**: Bug fixes applied to one copy but not others. Changes
  require updating multiple locations.
- **Fix**: Extract Function, Pull Up Method, Slide Statements, Form Template Method.

### Dead Code
- **Heuristic**: Unreachable code, unused variables, unused imports, commented-out
  code blocks, unused function parameters.
- **Why it matters**: Adds noise, confuses readers, increases bundle size.
- **Fix**: Remove Dead Code. Use IDE/linter warnings (TypeScript `noUnusedLocals`,
  ESLint `no-unused-vars`).

### Speculative Generality
- **Heuristic**: Abstractions, hooks, or parameters built for scenarios that never
  materialized — abstract classes with one subclass, unused function parameters,
  overly generic interfaces.
- **Why it matters**: Complexity without value. YAGNI — You Aren't Gonna Need It.
- **Fix**: Collapse Hierarchy, Inline Function, Inline Class, Change Function
  Declaration, Remove Dead Code.

### Lazy Element
- **Heuristic**: A function, class, or variable that does too little to justify its
  existence — a function that just calls another function, a class with one method.
- **Why it matters**: Unnecessary indirection.
- **Fix**: Inline Function, Inline Class, Collapse Hierarchy.

### Comments as Deodorant
- **Heuristic**: Comments that explain *what* code does rather than *why*. If the
  code needs a comment to be understood, the code should be refactored.
- **Fowler**: "When you feel the need to write a comment, first try to refactor
  the code so that any comment becomes superfluous."
- **Fix**: Extract Function (name it what the comment says), Change Function
  Declaration, Introduce Assertion.

---

## Couplers

Smells related to excessive coupling between modules.

### Feature Envy
- **Heuristic**: A function accesses another module's data or methods more than its
  own. It "envies" the other module's features.
- **Why it matters**: Logic is in the wrong place; changes to the envied module
  ripple unnecessarily.
- **Fix**: Move Function, Extract Function + Move.
- **Exception**: Strategy and Visitor patterns intentionally separate behavior from data.

### Insider Trading (Inappropriate Intimacy)
- **Heuristic**: Two modules exchange data too freely, creating tight coupling.
  One module reaches deep into another's internals.
- **Why it matters**: Changes to one module's internals break the other.
- **Fix**: Move Function, Move Field, Hide Delegate, Replace Subclass with Delegate.

### Message Chains
- **Heuristic**: Chains like `a.getB().getC().getD()` — client depends on the
  navigation structure between objects.
- **Why it matters**: Changes to any intermediate class break the chain.
- **Fix**: Hide Delegate, Extract Function, Move Function.

### Middle Man
- **Heuristic**: >50% of a class's methods just delegate to another class.
- **Why it matters**: Unnecessary indirection with no added value.
- **Fix**: Remove Middle Man, Inline Function, Replace Superclass with Delegate.

---

## Conditional Complexity

Smells related to complex branching logic.

### Nested Conditionals
- **Heuristic**: >2 levels of if/else nesting.
- **Fix**: Replace Nested Conditional with Guard Clauses (early returns).
  ```typescript
  // Smell
  if (user) {
    if (user.isActive) {
      if (user.hasPermission) {
        doThing()
      }
    }
  }
  // Better
  if (!user) return
  if (!user.isActive) return
  if (!user.hasPermission) return
  doThing()
  ```

### Repeated Switches
- **Heuristic**: Same switch/case or if/else chain appears in multiple places,
  branching on the same type or status field.
- **Fix**: Replace Conditional with Polymorphism — each variant gets its own
  class/strategy, or use an object map.
  ```typescript
  // Smell: scattered switch on userType
  // Better: const handlers = { admin: handleAdmin, user: handleUser }
  ```

### Complex Boolean Expressions
- **Heuristic**: Boolean expressions with 3+ clauses, especially with mixed
  AND/OR/NOT without clear grouping.
- **Fix**: Consolidate Conditional Expression — extract to a named function.
  ```typescript
  // Smell
  if (age > 18 && hasLicense && !isSuspended && country === 'US')
  // Better
  if (canDrive(applicant))
  ```

---

## DRY Violations

Patterns that violate "Don't Repeat Yourself."

### Magic Numbers and Strings
- **Heuristic**: Literal values used directly in logic without named constants.
  Same literal appears in 2+ places.
- **Fix**: Extract Constant with a descriptive name.
  ```typescript
  // Smell
  if (retries > 3) ...
  setTimeout(fn, 30000)
  // Better
  const MAX_RETRIES = 3
  const TIMEOUT_MS = 30_000
  ```

### Copy-Paste Variations
- **Heuristic**: Code blocks that are 80%+ identical with minor variations in
  values, field names, or types.
- **Fix**: Extract Function with parameters for the varying parts. If the
  variations are complex, use Combine Functions into Transform or Strategy pattern.

### Repeated Parameter Groups
- **Heuristic**: Same set of 3+ parameters passed to multiple functions.
- **Fix**: Introduce Parameter Object or Combine Functions into Class.

---

## Modern / TypeScript-React Specific Smells

### Prop Drilling
- **Heuristic**: Props passed through 3+ intermediate components that don't use them.
- **Fix**: Extract Context, use state management (Zustand, Jotai), or composition.

### God Component
- **Heuristic**: A single React component handles routing, data fetching, state
  management, and rendering.
- **Fix**: Extract Custom Hook (state/effects), Extract Component (UI sections),
  layered architecture.

### Props Copied to State
- **Heuristic**: `useState(props.value)` — copies props into local state, causing
  sync issues.
- **Fix**: Derive from props directly, or use `key` prop to force remount.

### Excessive `any` Types
- **Heuristic**: TypeScript's `any` used to bypass type safety instead of proper
  types, generics, or discriminated unions.
- **Fix**: Replace with proper types. Use `unknown` + type guards if truly dynamic.

### Boolean State Explosion
- **Heuristic**: Multiple boolean state variables that represent mutually exclusive
  states (`isLoading`, `isError`, `isSuccess`).
- **Fix**: Use discriminated unions:
  ```typescript
  // Smell
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<Error | null>(null)
  const [data, setData] = useState<Data | null>(null)
  // Better
  type State =
    | { status: 'idle' }
    | { status: 'loading' }
    | { status: 'success'; data: Data }
    | { status: 'error'; error: Error }
  ```

### Imperative Loops
- **Heuristic**: For/while loops doing map, filter, or reduce work.
- **Fix**: Replace Loop with Pipeline.
  ```typescript
  // Smell
  const results = []
  for (const item of items) {
    if (item.active) {
      results.push(item.name)
    }
  }
  // Better
  const results = items.filter(i => i.active).map(i => i.name)
  ```
