# Skeeper Configuration Reference

Authoritative reference for `.skeeper.yml`. Cross-checked against `internal/config/config.go`.

## File location and lifecycle

- `.skeeper.yml` lives at the repository root.
- It is committed to the main repository, alongside `skeeper.lock`.
- It is created by `skeeper init` and edited in place by `skeeper track`.
- It is decoded with `KnownFields(true)` — unknown keys produce a hard error, not a warning.

## Top-level schema

```yaml
sidecar: <git-remote-url> # required
bootstrap: <shell command> # optional; printed in init output for new contributors

settings: # optional block; all fields default
  guardrails:
    max_files: 100
    max_bytes: 10485760 # 10 MiB
  hooks:
    pre_push_timeout: 30s
    allow_skip_env: SKEEPER_SKIP

namespaces: # required; at least one entry
  - name: <namespace-name>
    patterns:
      - <doublestar glob>
    exclude:
      - <doublestar glob>
    respect_gitignore: true # optional; default true
```

## `sidecar`

Required. The Git URL of the sidecar repository. Trimmed but otherwise opaque to skeeper — it is passed straight to `git`. SSH (`git@github.com:owner/repo.git`) and HTTPS URLs both work; credential resolution follows the runner's normal Git configuration.

## `bootstrap`

Optional command string. Skeeper does not execute it; it is a hint surfaced in onboarding output and stored in the lockfile to help collaborators install the same `skeeper` build that produced the lock.

## `settings.guardrails`

| Field       | Default             | Meaning                                             |
| ----------- | ------------------- | --------------------------------------------------- |
| `max_files` | `100`               | Hard ceiling on the file count of any mutating plan |
| `max_bytes` | `10485760` (10 MiB) | Hard ceiling on the byte count of any mutating plan |

Both must be non-negative. A plan that exceeds either ceiling fails unless `--force` is passed.

## `settings.hooks`

| Field              | Default        | Meaning                                                    |
| ------------------ | -------------- | ---------------------------------------------------------- |
| `pre_push_timeout` | `30s`          | Wall-clock budget for the managed `pre-push` status check  |
| `allow_skip_env`   | `SKEEPER_SKIP` | Environment variable name that triggers the audited bypass |

`pre_push_timeout` accepts any `time.ParseDuration` value (`s`, `m`, `h`). `allow_skip_env` must be a valid environment variable name (uppercase letters, digits, `_`; cannot start with a digit).

## `namespaces[]`

Required. At least one entry. Skeeper expects unambiguous routing: every spec file is owned by exactly one namespace.

| Field               | Required | Meaning                                                                                                    |
| ------------------- | -------- | ---------------------------------------------------------------------------------------------------------- |
| `name`              | yes      | Slash-separated relative path; canonicalized by `path.Clean`                                               |
| `patterns`          | yes      | Doublestar globs; at least one                                                                             |
| `exclude`           | no       | Doublestar globs that prune files matched by `patterns`                                                    |
| `respect_gitignore` | no       | When `false`, this namespace ignores `.gitignore`/`.git/info/exclude`/global excludes (defaults to `true`) |

### Namespace name rules

- Must be relative (no leading `/`).
- Slash-separated only; backslashes are rejected.
- Each segment must consist of `[A-Za-z0-9._-]`.
- Segments cannot start with `.` and cannot equal Git internals (`head`, `config`, `hooks`, `index`, `info`, `logs`, `objects`, `packed-refs`, `refs`).
- The literal segment `__branches__` is reserved for branch-aware refs and rejected.

### Pattern rules

- Doublestar globs only; full syntax in [`bmatcuk/doublestar`](https://github.com/bmatcuk/doublestar) docs.
- Backslashes are normalized to forward slashes; leading `./` is stripped.
- Negative globs (leading `!`) are rejected. Use `exclude` instead.
- Duplicate patterns inside a single namespace are rejected.

### Ownership invariant

If two namespaces match the same file, the plan fails and the error message names the file. Resolve by adding an `exclude` entry to the namespace that should not own it. There is no order-based precedence.

### `respect_gitignore`

By default, every namespace honors:

- root `.gitignore`
- nested `.gitignore` at any depth
- `.git/info/exclude`
- the user's global `core.excludesfile`

`respect_gitignore: false` bypasses all of those for that namespace. `.git/` and `.skeeper/` are always excluded regardless of this flag.

## Local-only state files

These files are written under `.git/skeeper/` and are never committed:

| File               | Purpose                                             | Cleared by                                              |
| ------------------ | --------------------------------------------------- | ------------------------------------------------------- |
| `transaction.json` | Resumable mutating operation and phase              | successful `skeeper sync` or safe `skeeper repair`      |
| `bypass.json`      | Latest audited strict-hook bypass                   | next successful `skeeper sync` or `skeeper repair`      |
| `hydration.json`   | Locked sidecar blobs used for merge-aware restores  | refreshed by `skeeper pull`, `restore`, and `sync`      |
| `rescue/<id>/`     | Files moved aside before prune or overwrite actions | reported by `skeeper repair`; restored manually if used |

## Validation summary

Skeeper runs the following checks on every load:

1. `sidecar` is non-empty after trim.
2. At least one namespace is declared.
3. Each namespace name is unique and conforms to the segment rules above.
4. Each namespace declares at least one pattern.
5. Patterns and excludes parse as valid doublestar path patterns.
6. No `patterns[]` entry begins with `!`.
7. `pre_push_timeout` parses as a duration.
8. `allow_skip_env` is a valid environment variable name.
9. No unknown YAML keys appear anywhere in the document.

Failures surface from `skeeper init`, `skeeper sync`, `skeeper status`, and the managed hooks with the same error messages, so a config that round-trips through `skeeper init` will always be accepted by the runtime.
