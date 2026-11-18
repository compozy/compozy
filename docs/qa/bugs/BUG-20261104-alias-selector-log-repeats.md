# BUG-20261104-alias-selector-log-repeats: Missing alias selector warning repeats on every usage read

- Status: verified (fixed on fix/provider-usage-alias-selector, same PR)
- Impact (user-side): Cosmetic
- Severity: Low; Priority: P3
- Persona Affected: Ada
- Journey Step: Add a Claude alias without `CLAUDE_CONFIG_DIR` and read provider usage
- Scenarios: ET-provider-usage-surfaces
- Origin: #1266 QA walk on the integrated branch
- Report: docs/qa/reports/2026-11-04-provider-usage-runtime-alias.md

## Reproduction

Configure `claude-mm5` (`runtime_provider = "claude"`, `native_cli`) with no `CLAUDE_CONFIG_DIR` in its
command and start the daemon. Every target resolution — startup reconciliation, each
`compozy provider usage list`, each `GET /api/provider-usage` — logged another
`provider_usage.alias_selector_missing` WARN (11 lines within 13 seconds of startup plus two reads).
Expected: one warning per alias and selector change.

## Fix and re-walk

The resolver remembers the selector key it last reported per provider and logs again only when it
changes or after the alias passes the gate. Regression:
`TestProviderUsageResolver/Should_log_a_missing_alias_selector_once_per_change`. Re-walk after a
daemon restart, three CLI and three HTTP reads plus one refresh: exactly one warning, with
`selector_key=CLAUDE_CONFIG_DIR` and no path values.
