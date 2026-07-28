# Release notes

## Contents

1. Decide the flow
2. Flow A — `pr-release add-note`
3. Flow B — inline changelog from `git log`
4. The shared emoji legend
5. Worked examples

## 1. Decide the flow

The pre-flight detector reports `pr_release: true|false`.

- `true` → **Flow A**: the repo uses the `pr-release` CLI, so individual notes are written to `.release-notes/` and aggregated automatically when the release PR is cut.
- `false` → **Flow B**: there is no release tooling available, so generate an inline changelog block and embed it in the PR description's **Release Notes** section.

Never run both flows. Picking one keeps the source of truth singular.

## 2. Flow A — `pr-release add-note`

For each user-visible bullet produced by the impact exploration (`references/explore-impact.md`), create one note:

```bash
pr-release add-note \
  --title "Short, user-facing headline" \
  --type {feature|fix|breaking|highlight} \
  --body "Markdown explaining the change from the user's perspective."
```

Rules:

- `--type feature` — net-new capability a user can now do.
- `--type fix` — corrects user-visible broken behavior.
- `--type breaking` — requires user action (config rename, dropped support, API change). Always include a `--body` migration paragraph.
- `--type highlight` — non-fitting marquee item (re-architecture, performance milestone, ecosystem update).
- Omit `--body` to open `$EDITOR` instead of passing markdown inline; pass `--body` when scripting.
- Notes land in `.release-notes/*.md` (with YAML frontmatter: `title`, `type`). They are archived to `.release-notes/archive/vX.Y.Z/` when the next release branch is cut.

Do not edit the YAML frontmatter manually — the CLI maintains it. If a note needs revision, edit the body of the generated file.

## 3. Flow B — inline changelog from `git log`

When `pr-release` is unavailable, produce the changelog block inline and paste it into the PR description's **Release Notes** section. Steps:

```bash
BASE="$(gh pr view --json baseRefName -q .baseRefName 2>/dev/null || echo main)"
git log --pretty=format:'%s' "origin/${BASE}..HEAD"
```

Group the resulting subjects by the leading conventional-commit type, using the legend in section 4. Skip `chore(release):`, `wip`, and merge commits exactly as `cliff.toml` does.

Within each group, format each line as:

```
- *(scope)* Subject sentence (rest of the message, upper-cased first letter)
```

For commits without a scope, drop the `*(scope)*` prefix. For commits with a `BREAKING CHANGE:` footer, append an indented `- **BREAKING:** <description>` line under the bullet.

## 4. The shared emoji legend

These groups and emojis come from `pr-release/cliff.toml` and are the canonical mapping. Use them verbatim so Flow B output looks identical to Flow A's eventual aggregation:

| Conventional prefix          | Group heading              |
| ---------------------------- | -------------------------- |
| `feat`, `feature`            | 🎉 Features                |
| `fix`, `bugfix`              | 🐛 Bug Fixes               |
| `perf`                       | ⚡ Performance Improvements |
| `security`                   | 🔒 Security                |
| `docs`                       | 📚 Documentation           |
| `build`                      | 📦 Build System            |
| `ci`                         | 🔧 CI/CD                   |
| `refactor`                   | ♻️ Refactoring             |
| `test`                       | 🧪 Testing                 |
| `deps`, `chore(deps)`        | 📦 Dependencies            |
| `style`                      | 💅 Style                   |
| `chore` (non-release)        | 🔧 Miscellaneous Tasks     |
| `revert`                     | ⏪ Reverts                  |
| Anything else                | Other Changes              |

Skip entirely: `^chore\(release\):`, `^Merge branch`, `^Merge pull request`, `^Merge remote-tracking`, `^wip`, `^WIP`.

## 5. Worked examples

### Flow A — three notes for a feature PR

```bash
pr-release add-note --title "Multi-account auth" --type feature \
  --body "Users can now sign in with multiple workspaces simultaneously via the new account switcher in the navbar."

pr-release add-note --title "Session cookie shrunk to 4KB" --type highlight \
  --body "Internal: session payload trimmed from 18KB to under 4KB, eliminating CDN header-size failures for large orgs."

pr-release add-note --title "Drop support for Node 16" --type breaking \
  --body "Minimum Node version is now 18.18. Upgrade Node before installing this release; legacy installs will fail at \`npm install\`."
```

### Flow B — embedded changelog block

````markdown
## Release Notes

### 🎉 Features

- *(auth)* Multi-account workspace switcher in the navbar
- *(api)* Add `GET /v1/accounts` for listing accessible workspaces

### 🐛 Bug Fixes

- *(auth)* Session cookie no longer exceeds CDN header limits for large orgs

### ♻️ Refactoring

- *(session)* Move workspace state out of cookie into server-side store
  - **BREAKING:** Minimum Node version is now 18.18

### 📚 Documentation

- Update auth quick-start with multi-workspace flow
````

Keep the order of groups consistent with the legend table — features, fixes, perf, security, then everything else.
