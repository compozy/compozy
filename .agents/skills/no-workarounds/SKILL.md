---
name: no-workarounds
description: Enforce root-cause fixes over workarounds, hacks, and symptom patches in all software engineering tasks. Use when debugging issues, fixing bugs, resolving test failures, planning solutions, making architectural decisions, or reviewing code changes. Activates gate functions that detect and reject common workaround patterns such as type assertions, lint suppressions, error swallowing, timing hacks, and monkey patches. Don't use for trivial formatting changes or documentation-only edits.
---

# No Workarounds

## The Fundamental Law

```
A WORKAROUND IS A LIE TOLD TO THE COMPILER.
It makes the symptom disappear while the disease spreads.
```

A workaround is any change that makes a problem stop manifesting without addressing why the problem exists. Workarounds are not fixes. They are deferred failures with compound interest.

**Philosophical foundation:** Read `references/philosophical-foundations.md` for the engineering principles behind this skill, drawn from Toyota's Jidoka, Fowler's Technical Debt Quadrant, Torvalds' "good taste," and the Broken Windows Theory.

## The Workaround Detection Gate

```
BEFORE writing or proposing ANY fix:

1. STATE the problem clearly
2. ASK: "Why does this problem exist?" (not "How do I make it stop?")
3. TRACE to root cause (use systematic-debugging skill)
4. ASK: "Does my proposed fix address the ROOT CAUSE?"
5. ASK: "Would this fix be necessary if the code were correct?"
6. ASK: "Am I silencing a signal or fixing a source?"

IF any answer reveals symptom-patching:
  STOP — Redesign the fix to address root cause

IF root cause is in external code or truly unfixable:
  Document why, add defensive validation, and mark with WORKAROUND comment
  (See "The Escape Valve" section below)
```

## The Seven Categories of Workarounds

### Category 1 — TYPE: Type System Evasion

**The signal being silenced:** The type system is telling the code is wrong.

```typescript
// WORKAROUND: Lying to the compiler
const value = response.data as UserProfile;
const config = {} as AppConfig;
function process(input: any) { ... }

// PROPER FIX: Make types truthful
const value: UserProfile | undefined = response.data;
if (!value) throw new Error("Missing user profile");

const config: AppConfig = { theme: "light", locale: "en" };

function process(input: UserProfile) { ... }
```

**Gate function:**

```
BEFORE using `as`, `any`, `unknown` cast, or `!` (non-null assertion):
  Ask: "Why doesn't the type match?"

  IF the data shape is genuinely unknown:
    Use runtime validation (Schema, Zod, or type guards)
  IF the type is wrong:
    Fix the type definition
  IF the API returns unexpected shape:
    Fix the API contract or add a validation layer
  NEVER use type assertions to bypass compiler errors
```

### Category 2 — LINT: Lint and Warning Suppression

**The signal being silenced:** Static analysis found a real problem.

```typescript
// WORKAROUND: Shooting the messenger
// eslint-disable-next-line @typescript-eslint/no-unused-vars
const result = fetchData();

// @ts-ignore
someFunction(wrongArgs);

// @ts-expect-error - TODO fix later
brokenCall();

// PROPER FIX: Fix what the linter found
fetchData(); // Remove unused binding

someFunction(correctArgs); // Fix the arguments
```

**Gate function:**

```
BEFORE adding eslint-disable, @ts-ignore, @ts-expect-error, or any suppression:
  Ask: "What rule is being violated and WHY?"

  IF the code genuinely violates the rule:
    Fix the code, not the linter
  IF the rule is wrong for this codebase:
    Disable the rule in config (globally), not inline
  IF it's a third-party type issue:
    File an issue, add a minimal typed wrapper
  NEVER suppress a warning without understanding it
```

### Category 3 — SWALLOW: Error Swallowing

**The signal being silenced:** Something failed and the code pretends it didn't.

```typescript
// WORKAROUND: Pretending errors don't exist
try {
  await saveData(payload);
} catch {
  // silently ignore
}

try {
  result = JSON.parse(input);
} catch {
  result = {}; // default to empty - hides corrupt data
}

// PROPER FIX: Handle errors meaningfully
try {
  await saveData(payload);
} catch (error) {
  logger.error("Failed to save data", { error, payload });
  throw new SaveError("Data save failed", { cause: error });
}

const parsed = Schema.decodeUnknownSync(PayloadSchema)(input);
// Throws descriptive error if input is invalid
```

**Gate function:**

```
BEFORE writing a catch block:
  Ask: "What specific errors can occur here?"
  Ask: "What should happen when each error occurs?"

  IF the answer is "ignore it":
    STOP — Ignoring errors hides bugs
  IF the answer is "log it":
    Log AND propagate or handle meaningfully
  IF the answer is "use a default":
    Ensure the default is SAFE and the failure is LOGGED
  NEVER write an empty catch block
  NEVER catch Exception/Error broadly without re-throwing specific types
```

### Category 4 — TIMING: Timing and Lifecycle Hacks

**The signal being silenced:** Code runs in the wrong order or at the wrong time.

```typescript
// WORKAROUND: Racing against the clock
setTimeout(() => {
  element.focus();
}, 100);

await new Promise((resolve) => setTimeout(resolve, 500));
// "wait for state to settle"

await retry(() => checkCondition(), { times: 10, delay: 200 });
// retry loop hiding a race condition

// PROPER FIX: Fix the lifecycle
// Use framework-native lifecycle hooks
useEffect(() => {
  if (ref.current) ref.current.focus();
}, [isVisible]);

// Use proper event-driven coordination
await waitForEvent(emitter, "ready");

// Use condition-based polling (not blind retries)
await waitUntil(() => service.isReady(), {
  timeout: 5000,
  message: "Service failed to become ready",
});
```

**Gate function:**

```
BEFORE adding setTimeout, delay, sleep, or retry loops:
  Ask: "WHY is the timing wrong?"
  Ask: "What event signals that the system is ready?"

  IF there's an event or callback available:
    Use it instead of arbitrary delays
  IF the ordering is wrong:
    Fix the initialization order
  IF it's a test timing issue:
    Use condition-based waiting, never arbitrary sleeps
  NEVER use setTimeout(fn, 0) to "fix" rendering issues
  NEVER use arbitrary delays to "wait for things to settle"
```

### Category 5 — PATCH: Monkey Patching and Runtime Mutation

**The signal being silenced:** The API doesn't do what the code needs.

```typescript
// WORKAROUND: Mutating things you don't own
Array.prototype.customMethod = function () { ... };
Object.defineProperty(window, "fetch", { value: customFetch });
library.internals._privateMethod = replacement;

// PROPER FIX: Composition over mutation
function customOperation<T>(arr: T[]): T[] { ... }
const wrappedFetch = createFetchWrapper(window.fetch);
const adapter = new LibraryAdapter(library);
```

**Gate function:**

```
BEFORE modifying prototypes, globals, or third-party internals:
  Ask: "Does the library provide an extension point?"

  IF yes: Use the official extension mechanism
  IF no: Wrap with composition/adapter pattern
  IF the library is broken: File issue, fork, or find alternative
  NEVER modify objects the code doesn't own
```

### Category 6 — SCATTER: Defensive Duplication

**The signal being silenced:** The data is unreliable at its source.

```typescript
// WORKAROUND: Checking everywhere because source is broken
function renderUser(user: User) {
  const name = user?.name ?? user?.displayName ?? "Unknown";
  const email = user?.email ?? user?.contacts?.email ?? "";
  const id = user?.id ?? user?.userId ?? user?._id ?? "";
  // ... 20 more optional chains
}

// PROPER FIX: Validate once at the boundary
const user = Schema.decodeUnknownSync(UserSchema)(rawData);
// user is now guaranteed to have correct shape

function renderUser(user: User) {
  // No defensive checks needed — schema validated at entry
  return `${user.name} (${user.email})`;
}
```

**Gate function:**

```
BEFORE adding optional chaining (?.) or nullish coalescing (??) deeply:
  Ask: "Why might this value be missing?"
  Ask: "Where does this data enter the system?"

  IF data is unvalidated at entry:
    Add validation at the boundary, remove defensive checks downstream
  IF the type allows undefined but shouldn't:
    Fix the type to be non-optional
  IF it's truly optional:
    Handle the None/undefined case explicitly at the nearest decision point
  NEVER scatter optional chains as a substitute for proper validation
```

### Category 7 — CLONE: Copy-Paste Adaptation

**The signal being silenced:** The abstraction doesn't fit but the developer forces it.

```typescript
// WORKAROUND: Copy and "adapt" (badly)
// Copied from UserService and changed 3 lines
function createProject(data: ProjectData) {
  // 200 lines, 95% identical to createUser
  // but with subtle bugs from incomplete adaptation
}

// PROPER FIX: Extract shared pattern or write fresh
// Option A: Extract the common pattern
function createEntity<T>(schema: Schema<T>, repo: Repository<T>) {
  return (data: T) => pipe(
    Schema.decode(schema)(data),
    Effect.flatMap(repo.insert),
  );
}

// Option B: Write purpose-built code
function createProject(data: ProjectData) {
  // Clean, focused implementation for projects
}
```

**Gate function:**

```
BEFORE copying code and modifying it:
  Ask: "Am I copying because the pattern is the same or because I'm lazy?"

  IF the pattern is genuinely the same:
    Extract a shared abstraction first, then use it
  IF the pattern is similar but different:
    Write purpose-built code — similar-looking code with different intent
    should NOT be forced into the same abstraction
  NEVER copy-paste more than 5 lines without questioning why
```

## The Compound Cost

A workaround that saves 30 minutes today costs 30 hours when copied to 5 places, debugged 3 times, and confused 4 developers over 6 months. The interest rate on workarounds is predatory.

## Red Flags — STOP and Rethink

Catch these thought patterns and STOP:

| Thought                                      | What It Means                        |
| -------------------------------------------- | ------------------------------------ |
| "Just add `as any` to make it compile"       | TYPE — Type system evasion           |
| "Disable the lint rule for this line"        | LINT — Warning suppression           |
| "Wrap it in try-catch and ignore the error"  | SWALLOW — Error swallowing           |
| "Add a setTimeout to fix the timing"         | TIMING — Lifecycle hack              |
| "Override the prototype/global"              | PATCH — Monkey patching              |
| "Add `?.` everywhere just to be safe"        | SCATTER — Defensive duplication      |
| "Copy this code and change a few things"     | CLONE — Copy-paste adaptation        |
| "It works, don't touch it"                   | Fear masking a fragile workaround    |
| "We'll fix it properly later"                | Later never comes                    |
| "It's just temporary"                        | Nothing is more permanent            |

## The Escape Valve

Not every problem can be fixed at root cause. When a workaround is genuinely unavoidable:

```
REQUIRED conditions (ALL must be true):
  1. Root cause is in external code the team does not control
  2. The proper fix requires upstream changes with uncertain timeline
  3. The business impact of NOT shipping exceeds the technical debt cost
  4. The workaround is ISOLATED (does not leak into other code)

IF all conditions are met:
  1. Mark with explicit comment: // WORKAROUND: [reason] — see [issue-link]
  2. File a tracking issue for removal
  3. Add a test that verifies the workaround behavior
  4. Add a test that will FAIL when the upstream fix lands (canary test)
  5. Set a review date (max 90 days)

IF any condition is NOT met:
  Fix the root cause. No exceptions.
```

## Common Rationalizations

| Excuse                                         | Reality                                                      |
| ---------------------------------------------- | ------------------------------------------------------------ |
| "It's just a small workaround"                 | Small workarounds become big patterns when copied             |
| "We don't have time for the proper fix"        | Workarounds cost MORE time in debugging and maintenance       |
| "The type system is too strict"                | The type system found a real bug — listen to it               |
| "Nobody will copy this"                        | Every workaround in a codebase gets copied within 3 months   |
| "It's behind a feature flag"                   | Feature flags don't expire — the workaround becomes permanent |
| "The test passes"                              | A passing test with a workaround tests the workaround, not the code |
| "I'll create a tech debt ticket"               | 93% of tech debt tickets are never resolved                  |
| "The external library forces this"             | Use The Escape Valve process above, with all 5 requirements  |

## The Bottom Line

```
Every workaround is a bet that nobody will ever need to understand this code again.
That bet always loses.

Fix the disease, not the symptom.
Fix the source, not the signal.
Fix the code, not the compiler message.
```

**For the detailed catalog of 30+ specific workaround patterns with before/after code:** Read `references/workaround-catalog.md`.

**For the philosophical and engineering foundations:** Read `references/philosophical-foundations.md`.
