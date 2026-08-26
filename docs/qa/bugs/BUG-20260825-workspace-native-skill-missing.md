# BUG-20260825-workspace-native-skill-missing: A workspace-native skill disappears from installed management

- **Status:** verified
- **Impact (user-side):** Blocks-Completion
- **Severity:** Critical · **Priority:** P0
- **Persona Affected:** Bruno
- **Journey Step:** J-share-skills-with-other-tools, open a workspace skill and expose it
- **Scenarios:** ET-workspace-skill-source-teammate; ET-web-skill-expose-panel
- **Found:** 2026-08-25 · **Report:** docs/qa/reports/2026-08-25-skill-sources.md

## Summary

Bruno opens a repository containing a committed Compozy skill, but the installed Marketplace view
cannot find it. The skill exists on disk and workspace discovery measured it; only the resource
projection used by the installed catalog loses it.

## Reproduction

- **Charter:** CH-skill-expose-web-repair · **Tour:** Back-Button Tour
- **Environment:** desktop / 1280×900 / wifi-fast / en-US, isolated daemon-served runtime

1. Create `workspace-root/.compozy/skills/review-checklist/SKILL.md`.
2. Register and enter the workspace.
3. Open Marketplace > Skills and search installed skills for `review-checklist`.

**Expected:** the workspace-native skill appears and opens its installed detail.
**Actual:** the installed catalog omits it even though direct workspace discovery sees it.

## Evidence

- /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/skill-sources/origin-native-summary.json
- /Users/pedronauck/dev/qa-labs/compozy-skill-sources-final-rebased-20260825-20260825-230120-931206-lab/qa-artifacts/qa/browser-e2e

## Fix

- **Root cause:** workspace resource lookup had moved to the registered workspace id, while resolved
  skill roots were still published under the durable filesystem workspace identity. Publication and
  lookup therefore used different keys for the same workspace.
- **Fix commit:** `df739b0`
- **Regression test:** `internal/skills/registry_roots_test.go`,
  `TestWorkspaceResolvedSkillRoots/Should scope every root by registered workspace ID`.

## Verification

- **Retested:** 2026-08-25, same persona/journey · **Report:** docs/qa/reports/2026-08-25-skill-sources.md
- **Result:** E2E-011 opens the native skill from the installed catalog and completes its exposure,
  repair, and foreign-conflict lifecycle.
