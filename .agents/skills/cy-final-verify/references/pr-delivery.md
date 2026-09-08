# PR delivery

- Review task-owned changes before final validation and stage only those paths.
- Run the project's local gate or cite valid cached evidence, then commit/push and create or update the authorized PR.
- Monitor required checks at the current head; diagnose each finished failure immediately while other jobs continue. Repair the source, rerun the affected local checks, push, and follow the new head.
- Pending/red required checks remain in progress. If no required-check rules exist, use the project's policy for reported non-skipped checks. Cite the PR and head SHA for a delivery claim; do not substitute local full checks for CI.
