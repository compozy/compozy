---
description: Curated catalog of software engineering principles. Load this before analyzing code changes so you can map observations to named principles.
---

# Software Engineering Principles

Use this as a lookup table. When you spot a pattern in a diff, find the matching principle here. Each entry includes **code signals** -- what to look for in actual changes.

## Design Principles (SOLID)

**Single Responsibility Principle (SRP)**
A module should have one reason to change.
Code signals: A class/file was split into two. A function was extracted. A component stopped handling both UI and data fetching.

**Open/Closed Principle (OCP)**
Open for extension, closed for modification.
Code signals: New behavior added without changing existing code. A plugin/hook/callback system introduced. Strategy pattern or configuration used instead of conditionals.

**Liskov Substitution Principle (LSP)**
Subtypes must be substitutable for their base types.
Code signals: An interface was introduced to unify implementations. A subclass override changed behavior in a way that broke callers (violation). Type narrowing or guards added.

**Interface Segregation Principle (ISP)**
No client should depend on methods it doesn't use.
Code signals: A large interface was split into smaller ones. Optional methods removed from an interface. A "fat" props object was broken into focused ones.

**Dependency Inversion Principle (DIP)**
Depend on abstractions, not concretions.
Code signals: A concrete dependency replaced with an interface/injection. A factory or provider pattern introduced. Import paths changed from specific implementations to abstract layers.

**Composition over Inheritance**
Favor object composition over class inheritance.
Code signals: Inheritance hierarchy replaced with delegation. Mixins or HOCs replaced with hooks or composition. A "base class" was removed.

## Simplicity Principles

**DRY (Don't Repeat Yourself)**
Every piece of knowledge should have a single representation.
Code signals: Duplicate code extracted into a shared function. A constant replaced repeated literals. A template/generator replaced copy-pasted boilerplate.

**KISS (Keep It Simple, Stupid)**
The simplest solution that works is the best.
Code signals: A complex abstraction replaced with a straightforward implementation. Unnecessary indirection removed. A clever one-liner replaced with readable code.

**YAGNI (You Aren't Gonna Need It)**
Don't build it until you actually need it.
Code signals: Speculative features removed. Unused configuration options deleted. An over-engineered solution simplified to match actual requirements.

**Rule of Three**
Wait until the third duplication before abstracting.
Code signals: Similar code exists in 2 places and was left alone (good). A premature abstraction was introduced after only one use (violation). Third occurrence triggered extraction (textbook application).

**Principle of Least Surprise**
Code should behave the way most users would expect.
Code signals: A function renamed to better describe what it does. Return types made consistent. Side effects made explicit or removed.

## Structural Principles

**Separation of Concerns**
Different responsibilities should live in different modules.
Code signals: Business logic extracted from UI components. Data access separated from domain logic. Configuration separated from behavior. A "god file" split into focused modules.

**High Cohesion**
Related functionality should live together.
Code signals: Scattered related functions gathered into one module. A utility file broken up so each piece lives near its consumers. A feature folder created.

**Loose Coupling**
Modules should depend on each other as little as possible.
Code signals: Direct imports replaced with events/callbacks. A shared dependency removed. Modules communicate through well-defined interfaces instead of reaching into internals.

**Encapsulation**
Hide internal details, expose only what's necessary.
Code signals: Public methods reduced. Internal helpers made private. A module's API surface shrunk. Implementation details hidden behind a facade.

**Information Hiding**
Modules should not expose their internal data structures.
Code signals: Raw data structures wrapped in accessor methods. Internal state made private. A data transformation moved inside the module that owns the data.

## Pragmatic Principles

**Boy Scout Rule**
Leave the code better than you found it.
Code signals: Small cleanups alongside a feature change. A renamed variable for clarity. A dead code path removed. An outdated comment updated.

**Fail Fast**
Detect and report errors as early as possible.
Code signals: Input validation added at entry points. Assertions added for invariants. Early returns replacing deep nesting. Error handling moved closer to the source.

**Defensive Programming**
Anticipate and handle unexpected inputs gracefully.
Code signals: Null checks added. Default values provided. Edge cases handled. Error boundaries introduced.

**Premature Optimization (avoiding it)**
Don't optimize until you've measured.
Code signals: A simple implementation chosen over a "faster" complex one. Readability prioritized over micro-performance. A profiling step added before optimization work.

## Refactoring Patterns

**Extract Method/Function**
Code signals: A long function split into named sub-functions. Inline logic replaced with a well-named call.

**Extract Class/Module**
Code signals: A file split into multiple files. A class split into two with distinct responsibilities.

**Replace Conditional with Polymorphism**
Code signals: A switch/if-else chain replaced with a strategy pattern or subclass dispatch. A type map introduced.

**Introduce Parameter Object**
Code signals: Multiple related parameters grouped into a single options/config object. Function signatures simplified.
