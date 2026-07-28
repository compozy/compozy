---
name: repository-orientation
description: Map an unfamiliar repository before changing it. Use when a task crosses packages, public contracts, persistence, runtime wiring, or verification lanes and the owning architecture is not yet clear.
---

# Repository Orientation

Build a compact, evidence-backed map before proposing or editing code.

1. Read the repository and surface-specific instruction files that govern the requested paths.
2. Trace the user-visible entry point through contracts, runtime ownership, persistence, and downstream consumers.
3. Identify the invariant, owning layer, canonical test suite, and narrowest relevant verification lane.
4. Search for existing primitives and adjacent implementations before creating a new abstraction.
5. Record uncertainty explicitly; distinguish confirmed code paths from assumptions.

Return a concise orientation with:

- the goal and affected surfaces;
- the authoritative call and data flow;
- invariants and ownership boundaries;
- the working set of files and commands;
- unresolved questions that materially change implementation.

Do not begin a mutation until the map identifies the owner and a verification path.
