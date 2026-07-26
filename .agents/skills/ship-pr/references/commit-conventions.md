# Commit conventions

## Contents

1. Why this matters here
2. Detect the config
3. Read `type-enum` and `scope-enum`
4. Compose the commit message
5. Stage files safely
6. Hook failures
7. Common types

## 1. Why this matters here

Most repos in this workflow run [commitlint](https://commitlint.js.org/) in a pre-commit or commit-msg hook. A commit that fails commitlint is rejected; if the orchestrator does not respect the repo's config, the entire ship-pr flow halts mid-way. Detect the config once at the start, then follow it for every commit in this PR.

## 2. Detect the config

Probe in priority order — the first match wins (the pre-flight detector returns the path in its JSON output):

1. `commitlint.config.js`, `commitlint.config.cjs`, `commitlint.config.mjs`, `commitlint.config.ts`
2. `.commitlintrc`, `.commitlintrc.js`, `.commitlintrc.cjs`, `.commitlintrc.mjs`, `.commitlintrc.json`, `.commitlintrc.yaml`, `.commitlintrc.yml`
3. `package.json` with a top-level `"commitlint"` key
4. None of the above → fall back to `@commitlint/config-conventional` defaults (see section 7)

If multiple match, prefer the JS/TS forms over the JSON/YAML forms (they tend to extend a base config; JSON forms are usually shorthand snapshots).

## 3. Read `type-enum` and `scope-enum`

The two rules that constrain commit subject prefixes:

- `'type-enum'` — allowed `<type>` values (e.g., `feat`, `fix`, `chore`, `docs`).
- `'scope-enum'` — allowed `<scope>` values, often the repo's package or module names. Many repos leave scope unconstrained (`scope-enum` absent), which means any kebab-case scope is allowed.

Extract them from the config:

```bash
# JS / CJS / MJS / TS
node -e 'const c=require("./commitlint.config.cjs"); console.log(JSON.stringify(c.rules,null,2))'

# JSON
jq '.rules // ."commitlint" // .extends' .commitlintrc.json

# package.json
jq '.commitlint.rules' package.json
```

If the config only `extends` a preset (e.g., `["@commitlint/config-conventional"]`) without overriding the enums, the defaults in section 7 apply.

## 4. Compose the commit message

Use a HEREDOC to preserve formatting and avoid shell-quoting issues. Mandatory shape:

```
<type>(<scope>): <subject>          # ≤ 72 chars, imperative ("add", not "added")

<body — optional, wrapped at 72 chars, explains the why>

<footers — optional, e.g., BREAKING CHANGE: ..., Refs: #123>
```

Pattern:

```bash
git commit -m "$(cat <<'EOF'
feat(auth): allow multiple active workspaces per session

Adds a workspace-switcher in the navbar and a new
GET /v1/accounts endpoint listing accessible workspaces.

Co-Authored-By: Claude Opus 4.7 (1M context) <noreply@anthropic.com>
EOF
)"
```

Rules:

- Imperative subject. "add", "fix", "remove" — not "added", "fixes", "removes".
- Lower-case the type. `feat:`, never `Feat:`.
- Scope only when the repo uses scopes consistently. Inspect the last 20 commits with `git log --oneline -20` to learn the convention.
- For breaking changes, add a `BREAKING CHANGE:` footer with a migration paragraph. Prefer the footer over `feat!:` syntax unless the repo's existing log already favors the bang.

## 5. Stage files safely

Stage **by name**:

```bash
git add path/to/file-a.ts path/to/file-b.ts path/to/docs.md
```

Never use `git add -A`, `git add .`, or `git add -u`:

- They silently include unrelated files in `git status` (someone else's WIP, generated `.env`, build artifacts).
- They make follow-up reviews painful because the diff conflates concerns.

If the file list is large, group commits by concern. Multiple smaller commits in one PR are fine; one giant "everything" commit is not.

## 6. Hook failures

If `git commit` fails inside a hook (commitlint, lint-staged, format-staged, prettier, etc.):

1. Read the hook output verbatim. Hooks tell you exactly what is wrong.
2. Fix the underlying problem in the working tree (run the formatter, fix the lint error, rename the commit type).
3. Re-stage the corrected files.
4. Create a **new** commit. Do not `git commit --amend` — the previous commit may belong to unrelated work in the staging area, and `--amend` would silently merge them.
5. Never bypass with `--no-verify` or `--no-gpg-sign` unless the user has explicitly asked for it. The hook exists for a reason; bypassing it pushes the broken state to CI, where it will fail anyway and waste review time.

## 7. Common types

When no config is found or the config extends `@commitlint/config-conventional` without overrides, these are the allowed types:

| Type       | Use for                                                  |
| ---------- | -------------------------------------------------------- |
| `feat`     | A new user-visible feature                               |
| `fix`      | A bug fix                                                |
| `perf`     | A performance improvement that does not change behavior  |
| `refactor` | Code change that neither fixes a bug nor adds a feature  |
| `docs`     | Documentation-only changes                               |
| `test`     | Adding or correcting tests                               |
| `build`    | Build system, dependency manifests, lock files           |
| `ci`       | CI/CD configuration                                      |
| `chore`    | Routine maintenance (deps bumps not affecting users)     |
| `style`    | Code style / formatting only                             |
| `revert`   | Reverts a previous commit                                |

Pick the type that best matches the **dominant** intent of the commit. A feature PR may legitimately have a `chore(deps):` commit and a `feat(core):` commit — choose per commit, not per PR.
