# Spec Quality Markers

When the user opts into peer review, a spec (TechSpec, design doc, RFC, or detailed PRD) is
"ready for cross-LLM review" only when all six markers below are present. These markers are
tech-stack agnostic — they describe what any reviewable design must state, regardless of
language, framework, or datastore.

If a marker is missing, do not silently abort. Report the missing markers and ask the user
whether to amend the spec first or proceed anyway. External review on an incomplete spec
wastes credit and produces shallow noise — but the call is the user's.

## Marker 1: Scope / MVP Boundary Statement

The spec opens with an explicit boundary in plain language: which work composes the current
change, which follow-up work is deferred, and which features are intentionally out of scope.
A reader should be able to tell what is and is not being built without inference.

## Marker 2: Architectural Boundaries & Affected Components

A first-class section names which modules, packages, services, layers, routes, or shared
helpers may own the change. It explicitly calls out forbidden cross-layer dependencies,
wrapper bypasses, or coupling hazards when they matter.

## Marker 3: Concrete Interface / Contract Definitions

Critical interfaces, types, schemas, request/response payloads, or API contracts are shown
concretely, not waved at in prose. If a contract is still in flux, the spec says so
explicitly instead of pretending the shape is final.

## Marker 4: Data-Model & Migration Rationale

Any new or changed persistent data — columns, fields, indexes, constraints, enums,
backfills, config keys, or stored formats — is listed with its purpose, nullability/defaults,
and rollout implications (migration order, backward compatibility, data backfill). If no
persistence change is needed, the spec says why.

## Marker 5: Ownership & Pattern Decisions

For every new piece of state, data flow, cache, view-model, form, or filter concern, the spec
names the owning layer and the chosen pattern: local vs shared state, who loads/owns data,
reuse of an existing helper vs a new one, and canonical primitives vs inline copies. The point
is that the spec has made the decision, not deferred it to implementation guesswork.

## Marker 6: Safety / Verification Invariants Numbered

Security-sensitive, permission-sensitive, migration-sensitive, cache-sensitive, or
concurrency-sensitive behavior is spelled out as a numbered invariant list rather than loose
prose. The spec also names the verification surface that proves those invariants (unit /
integration / end-to-end / contract / lint / build).

If any of these markers is missing, surface the gap to the user before spending external
review credit.
