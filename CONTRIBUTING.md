# Contributing to Compozy

Thanks for your interest in contributing. This guide covers what you need to get started.

## Prerequisites

- Go 1.26+
- [golangci-lint](https://golangci-lint.run/) (used by `make lint`)
- A fork of [compozy/compozy](https://github.com/compozy/compozy)

## Getting Started

```bash
git clone git@github.com:<your-user>/compozy.git
cd compozy
git remote add upstream git@github.com:compozy/compozy.git
make verify   # must pass before any contribution
```

## Development Workflow

1. Sync your `main` with upstream before creating a branch.
2. Create a feature branch from `main`.
3. Make your changes.
4. Run `make verify` — this is the blocking gate. It runs `fmt → lint → test → build` in sequence. All four must pass with zero warnings and zero errors.
5. Push to your fork and open a PR against `upstream/main`.

### Verification

```bash
make verify    # full pipeline (required)
make fmt       # format with gofmt
make lint      # golangci-lint, zero tolerance
make test      # tests with -race flag
make build     # compile binary
make deps      # tidy and verify modules
```

Every PR must pass `make verify`. CI runs the same pipeline.

## Code Style

- **Files**: `kebab-case` with descriptive suffixes. Plural directory names for collections.
- **Types/Interfaces**: `PascalCase`. No "I" prefix for interfaces.
- **Functions/Variables**: `camelCase`. Booleans use `is`, `has`, `can`, `should`.
- **Constants**: `UPPER_SNAKE_CASE`.
- Wrap errors with context: `fmt.Errorf("context: %w", err)`.
- Use `errors.Is()` and `errors.As()` for error matching.
- No `panic()` or `log.Fatal()` in production paths.
- Use `log/slog` for structured logging.
- Pass `context.Context` as the first argument to functions crossing runtime boundaries.
- Design small, focused interfaces. Accept interfaces, return structs.
- Keep functions under 20 lines. Prefer early returns.

## Testing

- Table-driven tests with subtests (`t.Run`) as the default pattern.
- Use `t.Parallel()` for independent subtests.
- Use `t.TempDir()` for filesystem isolation.
- Mark helpers with `t.Helper()`.
- Mock via interfaces, not test-only methods in production code.
- Tests must pass with `-race`.

## Dependencies

Always use `go get` to add or update dependencies. Do not edit `go.mod` by hand.

## Skills

Compozy bundles skills in the `skills/` directory. Each skill has:

```
skills/<skill-name>/
  SKILL.md          # Skill definition with frontmatter
  references/       # Supporting persona and template files
```

When contributing a new skill or modifying an existing one:

- Follow the frontmatter format (`name`, `description`, `argument-hint`).
- Keep references self-contained within the skill's `references/` directory.
- Write all skill content in English.

## Commit Messages

- Use [Conventional Commits](https://www.conventionalcommits.org/): `feat:`, `fix:`, `refactor:`, `docs:`, `test:`, `chore:`.
- Keep the subject line under 72 characters.
- Focus on **why**, not what.

## Pull Requests

- Keep PRs focused. One logical change per PR.
- Write a clear description with a summary and test plan.
- Link related issues when applicable.
- All CI checks must pass before merge.

## License

By contributing, you agree that your contributions will be licensed under the [MIT License](LICENSE).
