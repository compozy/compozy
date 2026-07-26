# User Stories Template

Structure for `_user_stories.md` — the canonical user-story catalog that ships alongside `_prd.md`. Every story, acceptance criterion, and edge case for the feature lives here and only here; the PRD's User Stories section is an index into this file. Downstream consumers depend on it: `_techspec.md` maps stories to components, `_tests.md` builds its coverage matrix on story IDs, and review rounds validate the implementation against the acceptance criteria recorded here.

## ID Rules

- Stories are `US-NNN` (zero-padded, sequential). Acceptance criteria and edge cases are numbered within their story and referenced externally as `US-NNN.AC-N` and `US-NNN.EC-N`.
- IDs are permanent once written: downstream documents reference them, so never renumber or reuse an ID. Retire a dropped story by marking it `(withdrawn)` in the index instead of deleting the number.

## Document Skeleton

```markdown
# User Stories: [Feature Name]

Canonical behavior catalog for [feature]. Companion to `_prd.md`; consumed by
`_techspec.md` (component mapping) and `_tests.md` (coverage matrix).

## Personas

- **[Persona name]** — [who they are, their context, what they need from this feature]

## Story Index

| ID     | Feature Area | Persona   | Story                    |
|--------|--------------|-----------|--------------------------|
| US-001 | [area]       | [persona] | [one-line story summary] |

## [Feature Area 1]

### US-001: [Short title]

**As a** [persona], **I want** [capability], **so that** [outcome].

Acceptance criteria:

- AC-1: Given [starting context], when [action], then [observable result].
- AC-2: Given [context], when [action], then [observable result].

Edge cases:

- EC-1: [condition] → [expected behavior the user observes].
- EC-2: [condition] → [expected behavior].
```

## Edge-Case Sweep

Probe every story against every class below and record each finding as an `EC` entry with its expected behavior. Skip a class for a story only after actually probing it — most "cannot apply" verdicts turn out wrong, and an unswept class is how unhandled behavior reaches production.

| Class | Probe |
| --- | --- |
| Invalid input | Malformed, wrong type, out of range, unparseable, hostile. |
| Empty / missing | Empty collections, blank strings, absent optional data, first-run state. |
| Limits | Maximum sizes, quotas, truncation, pagination boundaries, rate limits. |
| Permissions | Unauthorized user, expired session, insufficient role, cross-tenant access. |
| Concurrency | Same action twice in flight, two actors on one resource, stale reads. |
| Interruption | Cancel mid-flow, connection loss, process restart, partial completion. |
| Repetition | Retry after success, duplicate submission, replay — is the action idempotent? |
| Ordering | Steps out of order, prerequisite skipped, back-navigation, deep links. |
| State transitions | Action on deleted/closed/archived entities, invalid state jumps. |
| Scale | Behavior at zero items, at typical volume, and at 100× typical volume. |

## Writing Rules

- Describe behavior the user observes, never implementation ("sees the last saved draft", not "reads from the drafts table").
- One story per capability. Splitting keeps acceptance criteria testable; merging stories to shorten the catalog hides behavior.
- Every AC must be checkable against the shipped product — someone can mark it true or false by using the feature.
- Every EC states condition **and** expected behavior ("upload over the size limit → rejected with a size-limit message", never just "large uploads").
- Give secondary personas (admin, operator, integrator) their own stories — most unhandled edge cases live in their flows.
