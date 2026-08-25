---
id: ET-skill-source-symlink-containment
area: ET
title: Keep linked skills to one entry and inside their own roots
persona: Dora
journey: J-diagnose-skill-sources
expected: A physical skill directory reached through links yields exactly one catalog entry attributed to its highest-precedence root, every escaping, dangling, or cyclic link is skipped with a per-entry diagnostic without failing the root, and no profile or workspace ever reads another one's roots through a link
entry_points: compozy skill sources; compozy skill sources -o json; compozy skill where <name>; compozy skill list --source <tier>; GET /api/settings/skills over HTTP or UDS; Settings > Skills per-root diagnostics at /settings/skills; compozy logs --type skills.scan.link_skipped --component skill
qa_status: untested
bug_ids:
fix_status:
retest_status:
fix_commits:
evidence:
last_report:
overlaps: ET-skill-source-diagnostics-cli; ET-skill-origin-attribution; ET-skill-exposure-lifecycle
---

Reproduce the installer layout that motivated this rule: one canonical skill body plus a per-tool
symlink pointing at it. With both presets enabled, the skill must appear exactly once — deduplicated
by resolved path, attributed to the highest-precedence root that reached it — and with only the
linking preset enabled it must still load, because first-level links are followed. Confirm the
attribution matches what `skill where` reports and does not flip between reads.

Then break the links deliberately, one class at a time, and confirm each is contained rather than
fatal. A link whose target was deleted is skipped with its own diagnostic and the root still finishes
scanning. A link resolving outside every trusted skill location is skipped and never followed. A
first-level cycle is detected and skipped, and the scan completes. Each skipped entry is attributed
to its root in the diagnostics and appears once in the ledger as `skills.scan.link_skipped` with its
path and reason.

Walk the isolation cases explicitly, because they are the ones a passing single-tenant run would
miss. A link inside one workspace's scanned folder that resolves into another workspace's configured
folder must be rejected with a diagnostic — workspaces never read each other's roots through links.
The same must hold between two profiles: a link reaching from one profile's projection into another
profile's roots is refused, and the two profiles never share a projection even when their source
configuration is identical. Confirm two workspaces and two profiles each get their own catalog rather
than one cached result reused across them.

Finish with CompozyOS's own links. Expose a skill into a scanned preset root, then rescan: the expose
link must never produce a second catalog entry and must never be recorded as shadowing its own
canonical skill. Confirm the per-root verification summary is present alongside the counts, so a root
that scanned more definitions than it published — because content verification blocked one — is
explainable rather than looking like a miscount.
