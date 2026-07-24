# Expected outcome — feat-payments

Exercises: deviation reconciliation (E2E-002) and malformed-ADR skip (E2E-019).

## Body (`.compozy/decisions/AD-NNN.md` for adr-001)

- `status: proven`, `source_adr: adrs/adr-001.md`, `tags` includes `payments`.
- `## Reconciliation` section contains a `[DEVIATION]` marker: plan said 24h TTL keyed on header;
  shipped a permanent (no-TTL) dedupe. Evidence cites the diff.

## Malformed ADR (`adrs/adr-002.md`)

- Skipped with a warning in the run summary. No `AD` created for it. The run still promotes adr-001.

## Assertions

- The deviation is explicit and evidence-cited, not silently normalized to the plan text.
- Exactly one promotion; the malformed file does not abort the run.
- A companion "implemented as designed" case (no `[DEVIATION]`) is covered by feat-orders.
