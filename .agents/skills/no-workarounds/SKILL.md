---
name: no-workarounds
description: Fix problems at their root cause instead of patching symptoms. Use when debugging, fixing bugs, resolving test failures, planning a solution, or reviewing a change — especially where a fix would silence a signal (type assertion, lint suppression, swallowed error, timing hack, monkey patch) rather than repair its source. Not for formatting- or docs-only edits.
metadata:
  author: Pedro Nauck
  github: https://github.com/pedronauck
  repository: https://github.com/pedronauck/skills
---

# No Workarounds

A workaround is any change that makes a problem stop manifesting without addressing why it exists. It makes the symptom disappear while the disease spreads — a deferred failure that compounds. **Fix the source, not the signal.**

## The gate — run before any fix

```
1. State the problem, then trace it to its root cause (use the systematic-debugging skill).
2. Does the fix repair that root cause, or only stop the symptom from showing?
3. Am I silencing a signal, or fixing a source?

Silencing a signal → redesign the fix against the root cause.
Root cause is external or genuinely unfixable → take the escape valve.
```

The fix is done when it would have been unnecessary had the code been correct in the first place — and it needs no cast, suppression, delay, or empty catch to pass.

## The seven signals

Each row is the compiler, linter, runtime, or reviewer telling you something true. Fix what it points at.

| Category | The signal it silences | Fix the source by… |
|---|---|---|
| **TYPE** — `as`, `any`, `!`, `as unknown as` | The type system found the code wrong | Making types truthful: correct the definition, or validate genuinely-unknown data at the boundary (Zod / Schema / type guard) |
| **LINT** — `eslint-disable`, `@ts-ignore`, `@ts-expect-error` | Static analysis found a real problem | Fixing what the rule flagged; if the rule is truly wrong for this repo, disable it in config, not inline |
| **SWALLOW** — empty catch, `.catch(() => null)`, catch-and-default | Something failed and the code pretends it didn't | Handling each error: log with context, then re-throw or map it to a typed result |
| **TIMING** — `setTimeout`, `sleep`, blind retry loops | Code runs in the wrong order | Coordinating on the real readiness event; in tests, wait on a condition, not the clock |
| **PATCH** — prototype / global / library-internal mutation | The API doesn't do what the code needs | Composing around it: wrapper, adapter, or the library's official extension point |
| **SCATTER** — deep `?.` / `??`, fallback chains | The data is unreliable at its source | Validating once at the boundary, then trusting the shape everywhere downstream |
| **CLONE** — copy-and-tweak of similar code | An abstraction doesn't fit but gets forced | Extracting the shared pattern, or writing purpose-built code |

**When any category's signal fires, read `references/workaround-catalog.md` in full before choosing the fix** — 30+ named patterns (W-01…W-30) with before/after code, including environment, build, test, and architecture workarounds beyond the seven above.

## The escape valve

Not every root cause is yours to fix. A workaround is allowed only when ALL hold:

```
1. The root cause is in external code the team does not control.
2. The proper fix needs upstream changes on an uncertain timeline.
3. The business cost of not shipping exceeds the debt incurred.
4. The workaround is isolated — it does not leak into other code.
```

When all four hold, contain it:

```
1. Mark it: // WORKAROUND: [reason] — see [issue-link]
2. File a tracking issue for its removal.
3. Add a test that pins the current behavior.
4. Add a canary test that FAILS once the upstream fix lands.
5. Set a review date (max 90 days).
```

If any condition fails, fix the root cause. No exceptions.

## Foundations & rationalizations

The principle converges from Toyota's Jidoka, Fowler's debt quadrant, Torvalds' "good taste," and Broken Windows — and every excuse for skipping it has a known answer. Read `references/philosophical-foundations.md`.
