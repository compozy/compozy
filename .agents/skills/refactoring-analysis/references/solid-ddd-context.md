# SOLID & DDD Context for Refactoring Analysis

## Important Disclaimer

SOLID principles have the most impact in **domain-rich, object-oriented codebases** —
particularly those using Domain-Driven Design (DDD), hexagonal architecture, or clean
architecture. In simpler projects (CRUD apps, utility libraries, scripts), applying
SOLID rigorously can lead to over-engineering.

> "SOLID impacta principalmente a inversão de dependência, extensibilidade, substituição
> sendo algo que vai funcionar em projeto orientado ao domínio, tipo DDD... pelo menos
> uma arquitetura hexagonal pra poder aplicar DIP... OCP, LSP e ISP muito em domain model."
> — Rodrigo Branas

**Rule**: Only recommend SOLID-based refactorings when the project has a complex domain
model with clear bounded contexts, entities, value objects, or domain services. Note
this context in the report.

---

## When SOLID Analysis Applies

Perform SOLID analysis when the project exhibits **at least 2** of:
- Domain entities or value objects (not just DTOs)
- Bounded contexts or explicit module boundaries
- Repository pattern or ports/adapters architecture
- Domain events or event-driven architecture
- Aggregate roots or domain services
- Hexagonal / clean / onion architecture layers

---

## SOLID Principles — Detection and Refactoring

### S — Single Responsibility Principle (SRP)

**Detection heuristics**:
- A class/module changes for multiple unrelated reasons (= Divergent Change smell)
- File has imports from many unrelated domains
- Class name includes "And", "Manager", "Handler" doing multiple things
- >5 public methods that group into 2+ unrelated clusters

**Refactoring**: Extract Class, Split Phase, Move Function

### O — Open/Closed Principle (OCP)

**Detection heuristics**:
- Adding a new variant (e.g., payment type, notification channel) requires modifying
  existing code instead of extending
- Switch/if chains on type codes that grow with each new variant
- Core logic mixed with variant-specific behavior

**Refactoring**: Replace Conditional with Polymorphism, Strategy Pattern,
Replace Type Code with Subclasses

**Context**: Most valuable in domain layers where new business rules and variants
are frequently added.

### L — Liskov Substitution Principle (LSP)

**Detection heuristics**:
- Subclass overrides a method to throw `NotImplementedError` or return `null`
  (= Refused Bequest smell)
- Subclass narrows the contract (rejects inputs the parent accepts)
- Subclass has side effects the parent doesn't specify

**Refactoring**: Replace Subclass with Delegate, Push Down Method, Extract Interface

### I — Interface Segregation Principle (ISP)

**Detection heuristics**:
- Interfaces with >7 methods where implementers stub or no-op several of them
- "Fat" interfaces that force unrelated capabilities together
- Classes implementing an interface but only using 2-3 of its methods

**Refactoring**: Extract Interface (split into focused interfaces), Role Interfaces

**Context**: Most relevant in domain model interfaces — repositories, services,
domain event handlers.

### D — Dependency Inversion Principle (DIP)

**Detection heuristics**:
- High-level domain modules directly importing low-level infrastructure
  (database drivers, HTTP clients, file system)
- Domain logic coupled to specific framework or library APIs
- No abstraction layer between domain and infrastructure
- Tests require spinning up real infrastructure instead of using test doubles

**Refactoring**: Extract Interface (port), Inject Dependencies, Introduce Adapter

**Context**: Requires at least hexagonal architecture to apply meaningfully.
In a flat Express/Next.js handler, DIP may be over-engineering.

---

## DDD-Specific Refactoring Opportunities

When the project uses DDD patterns, also evaluate:

### Aggregate Boundaries
- Aggregates that are too large (>5 entities) — consider splitting
- Aggregates that reference other aggregates by object reference instead of ID
- Cross-aggregate transactions that should be eventual consistency

### Value Objects
- Domain concepts represented as primitives that should be Value Objects
  (= Primitive Obsession, but in DDD context)
- Mutable objects that should be immutable Value Objects

### Domain Events
- Direct coupling between bounded contexts that should communicate via events
- Synchronous calls across context boundaries that should be async

### Anti-Corruption Layer
- External system models leaking into the domain
- Missing translation layer between contexts

---

## Report Template Additions for SOLID/DDD

When SOLID analysis is performed, add a section to the report:

```markdown
## SOLID Analysis

> **Context**: This project uses [DDD / hexagonal / clean] architecture with
> [bounded contexts / domain entities / etc.]. SOLID analysis is applicable.

### Findings

| Principle | Finding | Location | Severity | Recommendation |
|-----------|---------|----------|----------|----------------|
| SRP | ... | ... | ... | ... |

### Domain Model Health
- Aggregate boundaries: [assessment]
- Value object coverage: [assessment]
- Cross-context coupling: [assessment]
```

If SOLID analysis is NOT applicable, add:

```markdown
## SOLID Analysis

> **Skipped**: This project does not use domain-driven design or a layered
> architecture pattern. SOLID-specific analysis was not performed. For projects
> with complex business domains, consider adopting hexagonal architecture to
> benefit from SOLID principles — particularly Dependency Inversion (DIP) for
> testability and Open/Closed (OCP) for extensibility.
```
