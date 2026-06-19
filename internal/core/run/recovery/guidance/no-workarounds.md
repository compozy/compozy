No-workarounds requirements for recovery:

- Do not delete failing tests.
- Do not skip tests with `.skip`, `t.Skip`, focused-test filters, or equivalent bypasses.
- Do not weaken assertions to make a failure disappear.
- Do not add lint suppressions such as `eslint-disable`, `nolint`, `@ts-ignore`, or `@ts-expect-error`.
- Do not use type-system evasions such as `any`, unchecked casts, or non-null assertions to silence a real mismatch.
- Do not swallow errors or replace a real failure with an empty default.
- Fix the production root cause, not the signal that exposed it.
