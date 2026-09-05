# Spec Quality Markers

Use these markers to assess the requested review, not to add an approval stage. Scope and affected boundaries always apply. Interface definitions, persistent-data rationale, ownership decisions, and safety invariants apply only where the design changes them. A missing applicable contract is a review finding; an explicit request to review the saved draft is sufficient to proceed. Ask first only when missing scope makes a useful review impossible.

## Marker 1: MVP Boundary Statement

The spec opens with an explicit MVP boundary in plain language: which numbered tasks compose the MVP, which post-MVP work is deferred, and which features are explicitly out of scope.

Example (autonomy):
> "MVP boundary: tasks 01-16 implement the autonomy kernel. Tasks 17-18 prepare and execute QA. Post-MVP network evolution, broad memory scopes, self-correction telemetry, eval/replay, and broad web visibility remain follow-up specs unless explicitly pulled into scope later."

## Marker 2: Architectural Boundaries Section

A first-class `## Architectural Boundaries` section enumerates which packages can/cannot import which. Names new internal packages explicitly. References the `daemon/` composition root rule.

## Marker 3: Concrete Go Interface Signatures

Critical Go interfaces (e.g., `TaskClaimer`, `SpawnOpts`, `PermissionNarrower`) are pasted as code blocks, not described in prose. Every method signature is final.

## Marker 4: Data-Model Field Rationale

Any new SQLite columns, frontmatter fields, or config keys are listed with their purpose and shape. The spec explicitly forbids adding ownership state to JSON metadata blobs when a column or side table is appropriate.

## Marker 5: Side-Table vs JSON Decision

For every new domain entity that could be either a typed column/side-table or a JSON-bag entry, the spec names which choice and why. Side tables for matchable state; JSON for opaque metadata only.

## Marker 6: Lease / Safety Invariants Numbered

Concurrency- or ownership-sensitive code paths spell out invariants as a numbered list rather than prose. Example (autonomy lease invariants):

1. Exactly one active claim token per non-terminal run.
2. Heartbeat compares both run owner and claim token.
3. Stale/late after recovery fails explicitly.
4. Sweep + heartbeat serialize via SQLite tx.
5. Boot recovery before scheduler accepts wake/claim traffic.
6. Lease extension bounded by config.
7. One active lease per session in MVP.

Record material gaps and their consequence in the findings. Do not abort for inapplicable headings.
