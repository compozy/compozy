# BUG-20261107-plugin-archive-symlink-unsupported: Plugins whose repository contains symlinks cannot load

- Status: open
- Impact (user-side): Listed but not installable
- Severity: Medium; Priority: P2
- Persona Affected: Bruno
- Journey Step: Browse the default Claude plugin source
- Scenarios: ET-web-marketplace-landing-browse, ET-agent-plugin-marketplace-install
- Origin: spec marketplace-plugin-catalog S8 re-walk (isolated lab, 2026-11-07)

## Reproduction

After a refresh, 46 of 316 claude-plugins-official rows (and 1 compozy-plugins row) are `load_failed`.
39 of them report `pluginsource: extract repository snapshot: extract archive entry "…": unsupported
archive entry type 50` (a tar symlink), for example `data-engineering`
(`https://github.com/astronomer/agents.git` at a pinned `sha`). The rest: 3 manifest not found,
2 truncated archives (unexpected EOF), 1 file-count limit, 1 tar size limit, 1 unsupported source
kind. These rows list with an install blocker that the API reports as `trust_evidence_invalid`, and
web search shows them as Blocked. The diagnostic code is `unknown_component`.

Expected: symlinks contained in the archive are accepted (or skipped with a diagnostic) so the plugin
loads, and the blocker names the load failure.
