# TypeScript Modeling

Use this reference when TypeScript is available. It is one of the highest-leverage ways to make invalid states harder to represent.

## Strong defaults

- Prefer string literal unions over free-form strings.
- Prefer discriminated unions over nullable field soup.
- Narrow on `status` or another discriminant.
- Use exhaustiveness checking in `switch` statements.

## Example

Prefer this:

```ts
type SaveState =
  | { status: 'idle'; draft: Draft }
  | { status: 'saving'; draft: Draft }
  | { status: 'success'; draft: Draft; savedAt: string }
  | { status: 'failure'; draft: Draft; error: string };
```

Over this:

```ts
type SaveState = {
  isSaving: boolean;
  isSuccess: boolean;
  error: string | null;
  savedAt: string | null;
  draft: Draft;
};
```

The second shape permits many impossible combinations.

## When to use discriminated unions

Use them when:

- some fields only exist in certain modes
- status determines which actions are valid
- nullability is spreading because the model lacks explicit states

## Exhaustiveness

Prefer exhaustive `switch` statements so missing transitions fail loudly during development.

## Caution

Do not overcomplicate simple independent state with unions just because TypeScript can express them. Use the smallest type shape that protects the real invariants.
