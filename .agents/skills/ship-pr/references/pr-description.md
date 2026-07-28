# PR description

## Contents

1. Honesty contract
2. The base template
3. Section rules
4. When a repo ships its own PR template
5. Putting the body on disk before `gh pr create`

## 1. Honesty contract

Three rules that override every formatting preference:

1. **Never invent test results.** "Test plan" lists checks that actually ran during the session — nothing else. Unrun checks belong in a follow-up issue, not in the checklist as if they passed.
2. **Never describe features that are not in the diff.** Every Summary / Changes bullet must trace to a file in the diff.
3. **Never claim "no impact" without searching.** The impact-exploration step (`references/explore-impact.md`) produces the answer; do not skip it and write "no doc changes needed" by default.

## 2. The base template

```markdown
## Summary

- <one-sentence outcome from the user's perspective>
- <one sentence on the approach if it's non-obvious>
- <one sentence on follow-ups or scope deliberately left out>

## Changes

- **<impact area>**: <bullet from the explore-impact aggregation>
- **<impact area>**: <bullet>
- ...

## Release Notes

<embedded changelog block (Flow B) OR a one-line pointer "See `.release-notes/*.md` added in this PR" (Flow A)>

## QA  ← OMIT this section if no QA artifacts were detected

<see references/qa-artifacts.md>

## Test plan

- [x] <check that actually ran>
- [x] <check>
- [ ] <follow-up check tracked in issue #N>
```

The skeleton above is the *minimum*. Add sections (e.g., **Screenshots**, **Migrations**, **Rollout**) when they would help reviewers, but never add empty sections — reviewers read them as deceptive.

## 3. Section rules

- **Summary** — ≤ 3 bullets, ≤ 1 sentence each. Lead with the user-visible outcome, not the implementation. The first bullet should answer "what changes for someone using this".
- **Changes** — One bullet per impact area surfaced in `references/explore-impact.md`. Bold the area, then describe what changed. Keep code references as `path/to/file.ext:LN` when useful — reviewers can click them.
- **Release Notes** — In Flow A, link to the notes (e.g., "Adds `.release-notes/multi-account-auth.md`"); the release PR will surface them later. In Flow B, embed the full changelog block from `references/release-notes.md` here.
- **QA** — Only when artifacts exist. Defer to `references/qa-artifacts.md` for the exact structure.
- **Test plan** — Bulleted checklist with `[x]` for completed checks and `[ ]` for tracked follow-ups. Distinguish manual checks ("Reproduced login flow in Safari on macOS 14") from automated runs ("`pnpm test --filter @app/auth`").

## 4. When a repo ships its own PR template

Many repos have `.github/PULL_REQUEST_TEMPLATE.md`. `gh pr create` will pre-fill the body with it unless `--body` or `--body-file` is passed. To honor a repo template:

1. Read `.github/PULL_REQUEST_TEMPLATE.md` and use its section headings as the spine.
2. Map the content above onto those headings. The template's "Description" usually maps to **Summary** + **Changes**; its "Checklist" usually maps to **Test plan**.
3. Preserve any required checkboxes from the template (`- [ ] I have updated docs`, etc.). Tick them only if they are honestly true.

## 5. Putting the body on disk before `gh pr create`

`gh pr create --body "<huge string>"` is fragile across shells (quoting hell, command-length limits). Always write the body to a temp file:

```bash
BODY_FILE="$(mktemp -t ship-pr-XXXX.md)"
cat >"$BODY_FILE" <<'EOF'
## Summary
...
EOF
gh pr create --title "<title>" --body-file "$BODY_FILE"
rm -f "$BODY_FILE"
```

This also lets the orchestrator validate the body — e.g., grep for unfilled placeholders — before publishing the PR.
