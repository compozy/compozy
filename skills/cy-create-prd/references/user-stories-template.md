# User Stories Template

Structure for `_user_stories.md` — the canonical user-story catalog that ships alongside `_prd.md`. Every story, acceptance criterion, edge case, and authorization rule for the feature lives here and only here; the PRD's User Stories section is an index into this file. Downstream consumers depend on it: `_techspec.md` maps stories to components, `_tests.md` builds its coverage matrix on story and authorization-rule IDs, and review rounds validate the implementation against the behavior recorded here.

## ID Rules

- Stories are `US-NNN` (zero-padded, sequential). Acceptance criteria and edge cases are numbered within their story and referenced externally as `US-NNN.AC-N` and `US-NNN.EC-N`.
- Authorization rules are `AUTH-NNN` (zero-padded, sequential). Every rule references the `US-NNN.AC-N` or `US-NNN.EC-N` that defines its user-observable behavior.
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

### Uncertain-Outcome Recovery

[Include when a command can create durable side effects and the client can lose its terminal response. Link every row to the governing AC or EC.]

| Durable state | May execution begin or repeat? | Client response | Durable evidence | Retry after restart or transport failure |
| --- | --- | --- | --- | --- |
| `no record` | [yes/no and precondition] | [response] | [evidence proving no prior attempt] | [retry behavior] |
| `pending / incomplete` | [yes/no and recovery rule] | [response] | [evidence of incomplete work] | [retry behavior] |
| `completed success` | [yes/no] | [response or replayed result] | [terminal success evidence] | [retry behavior] |
| `completed failure` | [yes/no] | [response or replayed result] | [terminal failure evidence] | [retry behavior] |
| `fingerprint mismatch` | [yes/no] | [rejection] | [stored and presented fingerprints] | [retry behavior] |
| `corrupt / unreadable` | [yes/no] | [response] | [validation or read failure] | [retry behavior] |

## Authorization Rule Pack

| ID | Story / criterion | Operation | Protected resource / field | Data classification | Actor / role / capability | Outcome | Permitted side effects | Risk |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| AUTH-001 | US-001.EC-1 | [operation] | [resource or field] | [classification] | [actor, role, and required capability] | [outcome] | [explicit list or none] | [security-sensitive or lower-risk] |

Coverage basis:

- Security-sensitive: [dimensions covered by the complete matrix].
- Lower-risk: [documented pairwise coverage, selected pairs, and why omitted combinations are equivalent].
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
| Uncertain outcome | The client loses a terminal response after a potentially durable side effect — what evidence makes execution, replay, rejection, or `UNKNOWN_OUTCOME` safe? |
| Ordering | Steps out of order, prerequisite skipped, back-navigation, deep links. |
| State transitions | Action on deleted/closed/archived entities, invalid state jumps. |
| Scale | Behavior at zero items, at typical volume, and at 100× typical volume. |

## Uncertain-Outcome Recovery

When a command can create a durable side effect and its terminal response can be lost, the generic interruption and repetition probes are not sufficient. Add the recovery table from the document skeleton to the affected story and link each row to an AC or EC. Every row must define whether execution may begin or repeat, the exact client response, the durable evidence consulted, and retry behavior after process restart or transport failure.

Require the generated behavior to meet these safety rules:

- A `pending / incomplete` record uses documented incomplete-record recovery and is never treated as permission for a blind repeat.
- `completed success` and `completed failure` records use deterministic completed-result replay without repeating execution.
- A `fingerprint mismatch` uses mismatch rejection without execution or replay.
- A `corrupt / unreadable` record cannot authorize execution or replay. Return `UNKNOWN_OUTCOME` only when durable evidence cannot determine the result; otherwise return the result established by that evidence.
- A `no record` decision states what durable evidence proves absence and what must be persisted before execution may begin.

Give every applicable recovery row at least one downstream `_tests.md` obligation. The test set must cover deterministic completed-result replay, documented incomplete-record recovery, mismatch rejection without execution or replay, transport loss after commit, process restart in each nonterminal or terminal state that can survive restart, and `UNKNOWN_OUTCOME` only when durable evidence cannot determine the result.

## Authorization Rule Pack

Build the rule pack after the edge-case sweep. Include the section even when the feature has no protected behavior; in that case, write `None` and the concrete reason no operation or data needs authorization.

Inventory every applicable dimension before writing matrix rows:

- Operations: probe `create`, `read`, `update`, `delete`, `transition`, and `replay`; mark an operation non-applicable only with a reason.
- Protected data: name each resource and field, its Data classification using project terminology (for example public, internal, confidential, or restricted), and every client-controlled sensitive field.
- Principals: enumerate each Actor / role / capability combination that can reach the operation, including unauthenticated, expired, insufficient, and cross-tenant contexts where applicable.
- Outcomes: state whether the observable result is `allow`, `deny`, `redact`, or `ignore`.
- Effects: list the Permitted side effects for the row. For non-allow outcomes, default to none except explicitly required security auditing.

Use a complete matrix across every applicable operation × protected resource or field × actor, role, or capability combination for security-sensitive behavior. Security-sensitive includes tenant boundaries, secrets, personal or regulated data, financial data, privilege changes, destructive operations, lifecycle transitions, and durable side effects. Lower-risk behavior may use documented pairwise coverage only when the catalog lists the selected pairs, omitted combinations, and why those combinations are equivalent in risk and outcome.

Each `AUTH-NNN` row is a test obligation. Give it a source AC or EC with observable behavior, and require its own row with at least one test ID in downstream `_tests.md`; story-level coverage does not satisfy an authorization rule.

The catalog fails its generation gate if any condition below holds:

- Any protected operation is left without a negative test requirement covering `deny`, `redact`, or `ignore` as applicable.
- Any client-controlled sensitive field lacks its own authorization row and downstream test obligation.
- A `deny` outcome does not assert that protected state remains unchanged and that no unlisted side effect occurs. For `ignore`, identify the protected fields that remain unchanged even when other allowed fields change.
- Read rules rely only on endpoint access. When visibility differs by actor or capability, require field-level read redaction tests that name every visible and redacted field and the resulting response shape.
- Security-sensitive behavior uses sampling or pairwise coverage instead of the complete matrix.

## Writing Rules

- Describe behavior the user observes, never implementation ("sees the last saved draft", not "reads from the drafts table").
- One story per capability. Splitting keeps acceptance criteria testable; merging stories to shorten the catalog hides behavior.
- Every AC must be checkable against the shipped product — someone can mark it true or false by using the feature.
- Every EC states condition **and** expected behavior ("upload over the size limit → rejected with a size-limit message", never just "large uploads").
- Give secondary personas (admin, operator, integrator) their own stories — most unhandled edge cases live in their flows.
