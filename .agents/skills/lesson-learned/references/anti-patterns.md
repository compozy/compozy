---
description: Common anti-patterns to detect in code diffs. Use alongside se-principles.md for balanced analysis -- principles show what's good, anti-patterns show what to watch for.
---

# Anti-Patterns

When analyzing a diff, check for these signals. Present findings gently -- as opportunities, not failures.

## God Object / God Class

One module doing too much.
**Diff signals:** A single file with many unrelated changes. One class/module imported everywhere. A file over 500 lines that keeps growing.
**Suggest:** Extract responsibilities into focused modules (SRP).

## Shotgun Surgery

One logical change scattered across many files.
**Diff signals:** 10+ files changed for a single feature or fix. The same type of edit repeated in many places. A rename or config change touching dozens of files.
**Suggest:** Consolidate the scattered logic. If one change requires editing many files, the abstraction boundaries may be wrong.

## Feature Envy

A function that uses another module's data more than its own.
**Diff signals:** Heavy cross-module imports. A function reaching deep into another object's properties. Utility functions that only serve one caller in a different module.
**Suggest:** Move the function closer to the data it uses.

## Premature Abstraction

Abstracting before there are multiple concrete cases.
**Diff signals:** An interface with exactly one implementation. A factory that creates only one type. A generic solution for a problem that exists only once.
**Suggest:** Wait for the second or third use case before abstracting (Rule of Three).

## Copy-Paste Programming

Duplicated code blocks with minor variations.
**Diff signals:** Similar code appearing in multiple places in the diff. Functions that differ by only a parameter or two. Repeated patterns that could be parameterized.
**Suggest:** Extract shared logic, parameterize the differences.

## Magic Numbers / Strings

Literal values without explanation.
**Diff signals:** Hardcoded numbers in conditions (`if (retries > 3)`). String literals used as keys or identifiers. Timeouts, limits, or thresholds without named constants.
**Suggest:** Extract to named constants that explain the "why."

## Long Method

Functions that do too much.
**Diff signals:** New functions over 40-50 lines. Functions with multiple levels of nesting. Functions that require scrolling to read.
**Suggest:** Extract sub-steps into named functions. Each function should do one thing.

## Excessive Comments

Comments explaining "what" instead of "why."
**Diff signals:** Comments restating the code (`// increment counter`). Large comment blocks before straightforward code. Commented-out code left in place.
**Suggest:** Make the code self-documenting through better naming. Use comments only for "why" -- intent, trade-offs, non-obvious constraints.
