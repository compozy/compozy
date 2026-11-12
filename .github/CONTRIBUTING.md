# Contributing to CompozyOS

Thanks for helping build CompozyOS. This guide covers the basics — open an issue when something here is unclear.

## Before you build anything big

For new features, open an issue describing what you want to build **before** writing code. CompozyOS is in early alpha and the design moves fast — agreeing on direction first protects your time as much as ours. Bug fixes and small improvements can go straight to a pull request.

## Dev setup

You need Go 1.27.3+ (see `go.mod`) and Bun 1.4.0 (pinned in `.bun-version`).

```bash
bun install     # install JS workspace dependencies
make build      # compile the compozy binary
make test       # Go unit tests (race detector on)
make web-dev    # run the web UI locally
```

## Quality gates

- `make gate` — runs the checks affected by your diff. Run it before every push.
- CI runs the fast lane (preflight, lint, typecheck, unit, build) for affected areas on pull requests. Heavy lanes (integration, Windows and E2E) run on pushes to `main` and nightly; add `ci:full` and re-run CI, or change the CI pipeline, to run them on a pull request. There is no merge queue: merge-gate merges directly after its review criterion holds, even with pending or red CI. `main` may go red briefly and is fixed right after; every merge reports its non-green checks.
- `make gate-full` — optional local full verification when you want extra pre-push certainty.
- Lint is zero-tolerance: `make lint` (Go) and `bun run lint` (JS/TS) must pass with no warnings.
- A pre-commit hook formats staged files for you (oxfmt for JS/TS/YAML/Markdown, `make fmt` for Go).
- A pre-push hook checks format and lint on the files your branch changes, including type-aware Go lint on the changed packages. `COMPOZY_SKIP_PRE_PUSH=1` or `git push --no-verify` skips it.

## Commit messages

Format: `<type>: <description>` — type is one of `feat`, `fix`, `refactor`, `perf`, `docs`, `test`, `build`, `ci`. Don't use `chore` or `style`. A commit hook checks the format, and scopes are rejected — write `fix: …`, never `fix(web): …`.

```
fix: drop stale session sockets on daemon restart
docs: document workspace scoping for native tools
```

## Pull requests

- Keep each pull request focused on one change.
- Fill in the pull request template — especially how you verified the change.
- Behavior changes need tests. Changes to public behavior usually also need a docs update in `packages/site`.

### Investigating Go docstring coverage warnings

The [CodeRabbit configuration schema](https://storage.googleapis.com/coderabbit_public_assets/schema.v2.json)
marks `reviews.pre_merge_checks.docstrings` as deprecated: the Docstring Coverage check
has been retired and that setting has no effect. Historical reports used an 80% threshold;
do not interpret retirement as evidence that an earlier warning passed. The Go path
instructions still request evidence for documentation findings during ordinary review.

For a historical warning, an aggregate percentage is not a list of undocumented functions.
Expand **Full details:
Docstring Coverage** and **Review info** in the walkthrough, and retain the review URL,
reviewed head, compared commits, analyzed count and any tool failures.

Before editing comments, ask `@coderabbitai` on that PR to name the exact analyzed
functions, their file/line and receiver, and each covered/uncovered classification.
Ask whether the answer comes from retained analyzer evidence or a fresh manual estimate.
For each named function, inspect the attached Go comment at the reviewed head and
distinguish production code, test entry points, benchmarks and helpers. A comment inside
a body is not an attached Go documentation comment; a receiver method can have one just
like a package-level function. Do not assume the analyzer excludes tests or recognizes
every attached comment without evidence from that assessment.

Add a concise comment when it explains a missing contract, invariant or lifecycle rule.
Do not add name-restating boilerplate, lower the threshold, disable the check or exclude
tests to obtain a pass. If the evidence demonstrates a false positive, record the exact
function, existing comment, classification and reproduction links. An unnamed warning
or an unrelated tool timeout alone does not establish a false positive.

Request an explanation of the existing result on the unchanged reviewed head where
supported. Preserve that evidence before comparing a later assessment: a different diff
can change the denominator. If the service cannot recover its analyzed set, record that
limitation explicitly and keep the warning unresolved rather than claiming it repaired.
Path instructions guide review explanations; they cannot restore the retired check or
recover its historical analyzer inventory.

See [#1413](https://github.com/compozy/compozy/issues/1413) for the historical investigation,
the service's unchanged-head response and the remaining evidence limitations.

## Reporting bugs

Use the bug report form. `compozy version` prints the version line, and `compozy support bundle --yes` produces a secret-redacted diagnostics archive you can attach. Never paste unredacted logs or API keys.

## Security

Report vulnerabilities privately — see [SECURITY.md](../SECURITY.md).
