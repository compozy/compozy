# Main CI integration: loop authoring during validation

CI run `34389506603`, Web shard 2, failed the existing E2E-016 round-trip assertion: the published retry contained `backoff.base` but omitted `max_attempts: 3`. Its browser trace shows automatic validation starting at 308823 ms during the numeric fill; the field was disabled and empty immediately afterward. Background validation shared the same busy flag as writes and disabled authoring controls.

The editor now distinguishes pending writes from validation. Inspector, contract, and palette controls remain editable during validation; publication and validation actions retain their existing busy guards. Read-only ownership and pending writes still disable authoring.

The existing component suite owns the invariant that background validation cannot disable or discard edits. Its added regression holds the validation response pending, edits retry attempts and backoff, and checks the published request. It failed on the disabled input before the production change. The existing daemon-served E2E-016 remains unchanged and owns the real edit, publish, fresh-read, and run journey.

## Verification

- Root Turborepo Web test/build run: 7,107 tests in 771 files passed, including all 30 editor component tests; build passed.
- `make gate`: all affected lanes passed, including Web lint, typecheck, and tests.
- React Doctor on the changed React files: 100/100, no issues.
- Unchanged daemon-served E2E-016: three repetitions passed (7.1s, 7.4s, 6.5s; 3.3 minutes including worker builds), under the shared verification lock. Each fixture disposed its runtime. This verifies retry authoring, publication, fresh daemon read, and execution; it does not replace the other scenario walks listed in `LP-editor-authoring-walk`.

## Change impact

Following `docs/_memory/change-impact.md`: Web changes are confined to `/loops/:name/editor` controls and derived busy state; the affected scenario is `LP-editor-authoring-walk`. Native tool IDs, HTTP/UDS/CLI schemas, extensibility, hooks, configuration, and `skills/compozy/` remain unchanged because this fix only controls when browser inputs accept edits. Workspace identity still flows through the existing scoped editor store and API adapters; no storage shape or isolation boundary changes. Public site documentation requires no update.
