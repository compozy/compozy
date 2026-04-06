# Refactoring Techniques Catalog

Complete catalog of refactoring techniques from Martin Fowler's "Refactoring" (2nd Edition).
Organized by chapter and category. Use this as a reference when mapping smells to fixes.

---

## Quick Reference: Smell → Technique

| Smell | Primary Technique(s) |
|-------|---------------------|
| Long Function | Extract Function, Decompose Conditional, Split Loop |
| Duplicated Code | Extract Function, Pull Up Method, Slide Statements |
| Long Parameter List | Introduce Parameter Object, Preserve Whole Object, Remove Flag Argument |
| Feature Envy | Move Function, Extract Function + Move |
| Data Clumps | Extract Class, Introduce Parameter Object |
| Primitive Obsession | Replace Primitive with Object, Discriminated Unions |
| Large Class/Module | Extract Class, Extract Superclass, Split Phase |
| Repeated Switches | Replace Conditional with Polymorphism, Object Map |
| Message Chains | Hide Delegate, Extract Function |
| Nested Conditionals | Replace Nested Conditional with Guard Clauses |
| Dead Code | Remove Dead Code |
| Magic Numbers | Extract Constant |
| Mutable Shared State | Encapsulate Variable, Split Variable |
| Imperative Loops | Replace Loop with Pipeline (map/filter/reduce) |
| Middle Man | Remove Middle Man, Inline Function |
| Shotgun Surgery | Move Function, Move Field, Combine Functions into Class |
| Divergent Change | Extract Class, Split Phase |
| Speculative Generality | Inline Function, Collapse Hierarchy, Remove Dead Code |
| Comments as Deodorant | Extract Function (name = comment), Rename Variable |
| Temporary Field | Extract Class, Introduce Special Case |

---

## A First Set of Refactorings

### Extract Function
The most common refactoring. Pull a code fragment into its own function named
after what it does. Even 2-3 lines benefit if naming adds clarity.
- **When**: Code needs a comment to explain, or logic is at a different
  abstraction level than surrounding code.
- **Reverse**: Inline Function.

### Inline Function
Remove a function and put its body back into callers.
- **When**: Function body is as clear as its name, or excessive indirection.

### Extract Variable
Give a complex expression a self-describing name.
- **When**: An expression is hard to understand at a glance.
- **Reverse**: Inline Variable.

### Change Function Declaration
Rename a function, add/remove parameters, or change parameter types.
- **When**: A name doesn't communicate intent, or the parameter list needs adjustment.

### Encapsulate Variable
Wrap data access behind getter/setter functions.
- **When**: Widely accessed mutable data needs controlled access.

### Introduce Parameter Object
Group recurring parameter clusters into a single object.
- **When**: Same 3+ parameters appear together in multiple function signatures.

### Combine Functions into Class
Group functions that operate on the same data into a class or module.
- **When**: Several functions pass the same data between each other.

### Combine Functions into Transform
Gather derived-data calculations into a single transform function.
- **When**: Multiple places compute derived values from the same source data.

### Split Phase
Separate code into phases with distinct responsibilities, connected by a
data structure passed between phases.
- **When**: Code does two different things — e.g., parsing + calculation.

---

## Encapsulation

### Encapsulate Record
Replace raw objects/records with classes that control access.
- **When**: Data structure is widely used and changes need to be tracked.

### Encapsulate Collection
Return copies or read-only proxies instead of raw collections.
- **When**: Internal collection can be mutated by external code.

### Replace Primitive with Object
Wrap a primitive in a domain-specific type.
- **When**: A primitive carries domain meaning (money, email, phone, ID).
- **TypeScript**: Use branded types or Zod schemas.

### Replace Temp with Query
Convert a temporary variable to a function call.
- **When**: A temp is computed once and used in multiple places.

### Extract Class
Split a class with multiple responsibilities into two.
- **When**: A subset of fields/methods form a coherent group.

### Hide Delegate
Create wrapper methods to prevent clients from navigating through an object chain.
- **When**: Client code uses `a.getB().getC()` chains.
- **Reverse**: Remove Middle Man.

### Substitute Algorithm
Replace a method body with a clearer algorithm.
- **When**: There's a simpler way to do the same thing.

---

## Moving Features

### Move Function / Move Field
Move a function or field to the module that uses it most.
- **When**: A function accesses more data from another module than its own.
- **The bread and butter of refactoring** — Fowler.

### Move Statements into Function
Merge duplicated setup/teardown into the called function.
- **When**: Same statements always precede or follow a function call.

### Replace Inline Code with Function Call
Replace code with a call to an existing function that does the same thing.
- **When**: Code duplicates what a library or utility function already does.

### Slide Statements
Move related code lines together.
- **When**: Related declarations or logic are scattered within a function.

### Split Loop
Separate a loop that does two things into two loops.
- **When**: A loop computes two unrelated things in one pass. Clarity > micro-optimization.

### Replace Loop with Pipeline
Replace imperative loops with `map`/`filter`/`reduce`/`flatMap`.
- **When**: A loop initializes a result, iterates, conditionally adds to result.

### Remove Dead Code
Delete unreachable or unused code.
- **When**: Code is never called. Version control remembers it if needed.

---

## Simplifying Conditional Logic

### Decompose Conditional
Extract condition, then-branch, and else-branch into named functions.
- **When**: A conditional block is complex enough to need comments.
```typescript
// Before
if (date.before(SUMMER_START) || date.after(SUMMER_END))
  charge = quantity * winterRate + winterServiceCharge
else
  charge = quantity * summerRate
// After
if (isSummer(date))
  charge = summerCharge(quantity)
else
  charge = winterCharge(quantity)
```

### Consolidate Conditional Expression
Combine related conditions into a single named check.
- **When**: Multiple conditions yield the same result.

### Replace Nested Conditional with Guard Clauses
Use early returns for edge cases, keeping the happy path un-nested.
- **When**: >2 levels of if/else nesting.
- **Key insight**: Two flavors — (1) both branches equally likely: use if/else,
  (2) one is the "normal" path: use guard clauses.

### Replace Conditional with Polymorphism
Replace switch/if chains with subclass or strategy overrides.
- **When**: The same conditional appears in multiple places.
- **Modern alternative**: Object maps or function maps.
```typescript
const handlers: Record<Status, Handler> = {
  pending: handlePending,
  active: handleActive,
  closed: handleClosed,
}
handlers[status]()
```

### Introduce Special Case (Null Object)
Replace scattered null/undefined checks with a Special Case object.
- **When**: Many places check for the same special value.

### Introduce Assertion
Make implicit assumptions explicit with assertions.
- **When**: Code assumes a condition but doesn't verify it.

---

## Refactoring APIs

### Separate Query from Modifier
Functions should either return a value OR have side effects — not both.
- **When**: A function both modifies state and returns a value.

### Remove Flag Argument
Replace boolean/flag parameters with separate explicit functions.
- **When**: A function takes a boolean that changes its behavior.
```typescript
// Smell
setDimension(name, value, isMetric)
// Better
setMetricDimension(name, value)
setImperialDimension(name, value)
```

### Preserve Whole Object
Pass the whole object instead of extracting several values from it.
- **When**: Multiple values from the same object are passed as separate parameters.

### Replace Constructor with Factory Function
Use a factory when construction needs flexibility.
- **When**: Constructor limitations (naming, return type) are problematic.

---

## Dealing with Inheritance

### Pull Up Method / Pull Up Field
Move shared method/field from subclasses to superclass.
- **When**: Multiple subclasses have the same method/field.

### Push Down Method / Push Down Field
Move method/field from superclass to specific subclass.
- **When**: Only one subclass uses the method/field.

### Replace Subclass with Delegate
Use composition instead of inheritance for variation.
- **When**: Subclassing creates coupling or the "is-a" relationship doesn't hold.
- **Modern preference**: Favor composition over inheritance.

### Replace Superclass with Delegate
Replace inheritance with delegation.
- **When**: The "is-a" relationship is incorrect (e.g., Stack extends List).

### Collapse Hierarchy
Merge a superclass and subclass that are too similar.
- **When**: A subclass adds no meaningful behavior.

---

## When to Refactor (Fowler's Workflows)

### Preparatory Refactoring
"Make the change easy, then make the easy change." Restructure before adding a feature.

### Comprehension Refactoring
When reading code, refactor to make the understanding explicit.

### Litter-Pickup Refactoring
"Always leave the code better than you found it." (Boy Scout Rule)

### The Rule of Three
First time: just do it. Second time: wince. Third time: refactor.

### Long-Term Refactoring
Large changes spanning weeks. Use Branch by Abstraction to keep the system working.

### Fowler's Core Position
"Refactoring is not an activity you set aside time to do. Refactoring is something
you do all the time in little bursts."
