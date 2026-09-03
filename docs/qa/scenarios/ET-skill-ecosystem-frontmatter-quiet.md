---
id: ET-skill-ecosystem-frontmatter-quiet
area: ET
title: Load another tool's skill definitions without a wall of warnings
persona: Dora
journey: J-diagnose-skill-sources
expected: Skill definitions authored for other tools load silently with their ecosystem fields recognized and visibly not honored, while a genuinely unknown field still raises a warning that names it
entry_points: compozy skill sources; compozy skill sources -o json; compozy skill info <name>; compozy skill view <name>; Settings > Skills per-root diagnostics at /settings/skills; daemon logs during a scan pass
qa_status: pass
bug_ids:
fix_status:
retest_status: pass
fix_commits:
evidence: /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/list-additional.json;/Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/logs-skill-cli.json;/Users/pedronauck/dev/qa-labs/compozy-skill-terminal-recovery-20260902-205559-939126-lab/qa-artifacts/qa/test-cases/malformed-skill-recovery.md
last_report: docs/qa/reports/2026-09-02-skill-terminal-recovery.md
overlaps: ET-skill-source-diagnostics-cli; ET-skill-source-symlink-containment
---

Enable a source whose skills were written for another tool and populate it with real ecosystem
definitions — the tool-restriction, invocation-hint, and model-preference fields those tools
document. Turning the source on must not produce a warning per field per skill. The point is not
tidiness: diagnostics are the surface an operator reads when a skill is missing, and a wall of
warnings about fields Compozy deliberately ignores makes the one real warning unfindable.

Confirm the fields are recognized *and* inert. A tool-allowance field that reads like a permission
grant must never be enforced by CompozyOS — check that invoking the skill neither widens nor narrows
what the session can do based on it, and that the skill's inspection surfaces do not present it as an
active setting. Recognized-but-inert is the contract; silently honoring another tool's permission
vocabulary would be the dangerous outcome.

Then prove the signal survives. Add a definition carrying a genuinely unknown field and confirm a
warning still identifies that field by name. Add a definition missing the required name field and
confirm it is rejected with its own per-skill diagnostic while the other skills in the same root load
normally. Add a directory with no `SKILL.md` inside the root and confirm it is ignored rather than
treated as an error.

Read the result from both the human table and the structured output, and confirm the per-root
verification summary explains any gap between what was scanned and what was published — so "scanned
five, shows three" is always answerable from the product rather than from the logs.

QA impact 2026-09-02: reset because malformed skill definitions are now withheld from the workspace
catalog instead of falling back to their directory name, and the scanner no longer emits a repeated
warning for the same malformed file. Re-walk the malformed-neighbor, structured diagnostics, repair,
and quiet-rescan path. Charter: `CH-malformed-skill-recovery`.
