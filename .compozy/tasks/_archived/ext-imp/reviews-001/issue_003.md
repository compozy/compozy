---
status: resolved
file: internal/cli/form_test.go
line: 409
author: coderabbitai[bot]
provider_ref: thread:PRRT_kwDORy7nkc56T0iO,comment:PRRC_kwDORy7nkc624f7h
---

# Issue 003: _⚠️ Potential issue_ | _🟡 Minor_
## Review Comment

_⚠️ Potential issue_ | _🟡 Minor_

**Assert the rendered select options, not just the backing catalogs.**

These subtests currently pass as long as the overlay entries make it into `agent.DriverCatalog()` / `provider.Catalog(...)`, even if `addIDEField` or `addProviderField` later filters them out. Please assert against the actual field contents so the test protects the form behavior it names.


As per coding guidelines, "Ensure tests verify behavior outcomes, not just function calls."

<details>
<summary>🤖 Prompt for AI Agents</summary>

```
Verify each finding against the current code and only fix it if needed.

In `@internal/cli/form_test.go` around lines 368 - 409, The tests currently check
driver/provider catalogs instead of what the form actually renders; update the
two subtests that call addIDEField and addProviderField to locate the added
field in builder.fields (e.g., inspect builder.fields[0] or search for the field
by type/name), then assert that that field's rendered select options include the
expected entries ("ext-adapter" with display "Mock ACP" for addIDEField and
"ext-review" with display "Extension Review" for addProviderField) and that the
options are what a user would see (not just present in
agent.DriverCatalog()/provider.Catalog()); keep the existing checks that exactly
one field was registered and use the same selected variable passed to
addIDEField/addProviderField to validate option population.
```

</details>

<!-- fingerprinting:phantom:medusa:grasshopper:7d02b572-202f-4827-b1fe-ee0590997543 -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Triage

- Decision: `valid`
- Notes:
  - Root cause: the test only checks the backing overlay catalogs, so it would not catch regressions where `addIDEField` or `addProviderField` stopped rendering those choices into the form.
  - Fix plan: assert against the actual built field output while preserving the single-field registration checks.
  - Resolved: updated `internal/cli/form_test.go` to assert on rendered select-field output; verified with `make verify`.
