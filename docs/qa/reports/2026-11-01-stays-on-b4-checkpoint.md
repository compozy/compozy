# B4 implementation checkpoint

Pending-interaction expiry is stored by additive global migration 00163. ACP
resolves the wait once per interactive request and projects the deadline through
session storage and API DTOs. Generated OpenAPI and clients include `expires_at`.

Passed: focused ACP tests, SQLite `TestPendingInteractionDescendants`,
`make codegen`, and `make codegen-check` before excluding unrelated Open Design
generator drift from the checkpoint.

The canonical global fresh/reopen/ahead test exceeded its unchanged 180-second
budget twice: migration 00128 in the grouped run and 00153 when isolated outside
the sandbox. Both failures occurred before migration 00163. Full migration
validation remains open; this checkpoint is not a delivery claim.

Remaining: native bridge deadline alignment, native pending-interactions tool,
transactional expiry/interruption reports, supervision evidence, initiator
migration and propagation, unattended observe filters/rollups, and final QA.

## Transactional interaction reports

The canonical pending-interaction transition now inserts `approval_expired`
(`interaction:<id>`) for permission timeouts and `interrupted` (`turn:<id>`)
for restart orphaning/restart cancellation when a turn ID exists. It resolves
instance lineage inside the transaction and skips archived instances.
`TestPendingInteractionReportsAtomicity` uses real SQLite and a child session;
report-insert and subsequent transition failures roll back, and retries produce
one report. The focused race run passed. Boot interruption for turns without
pending interactions and native cancellation/deadline alignment remain open.

## Held permission supervision

The session manager exposes pending permission interactions as `held_permission`
work signals. Resolution removes the signal; orphaned/resolved interactions do
not supply live evidence. The owning supervision suite passed with race checks,
including an eleven-hour simulated wait, quiet-warning suppression, signal
removal, existing operator waits, and idle runtime release. Public enum clients
were regenerated and Web maps the source to "pending permission".

The focused root Turbo Web typecheck passed after updating existing interaction
fixtures with the required session identity; its codegen-check and client build
dependencies passed as well. Live QA remains pending.

## Initiator persistence

Global migration 00164 appends `initiator_kind` and `initiator_id` to sessions
and permission_log, with empty defaults for unknown historical identity.
Permission insertion snapshots the session identity in the same SQL statement;
subsequent session changes do not rewrite the audit row. The focused SQLite
snapshot/default tests passed with race detection, and codegen/codegen-check
passed before excluding unrelated Open Design generator drift. Session creation
attribution, root inheritance, public DTOs, and unattended observe remain open.

## Initiator propagation

Session creation now carries the structured origin kind and optional ID into
runtime state, metadata, and the catalog insert. Children copy the root's
initiator, and resume restores the persisted identity. Unknown/missing historical
roots remain unknown. Focused creation/resume/two-generation inheritance tests
and catalog registration/audit tests passed with race detection. Origin IDs at
external ingress points and public DTO/observe exposure remain open.
