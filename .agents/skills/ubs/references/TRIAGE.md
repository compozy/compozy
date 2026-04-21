# Triage Guide

## Quick Reference

| Severity | Action | Categories |
|----------|--------|------------|
| **Critical** | Fix before commit | 1-5 (null, security, async, leaks, coercion) |
| **High** | Fix before merge | 6-10 (division, resources, errors, promises, mutation) |
| **Medium** | Address or justify | 11-14 (debug, TODO, `any`, readability) |

```
                 HIGH CONFIDENCE          LOW CONFIDENCE
                 ─────────────────────────────────────────
HIGH SEVERITY    Fix immediately          Investigate first
                 Never suppress           then fix or justify
                 ─────────────────────────────────────────
LOW SEVERITY     Fix if easy              Document and defer
                 defer if complex         to future cleanup
```

---

## Tier 1: Fix Before Commit

### 1. Null Safety
Runtime crash on real data.

```javascript
// CRASH
user.profile.name;

// FIXED
user?.profile?.name ?? 'Anonymous';
```

**Suppress if:** Validated by caller/API/schema. Document: `// ubs:ignore — validated by [source]`

### 2. Security
XSS, injection, secrets, prototype pollution.

| Pattern | Risk | Fix |
|---------|------|-----|
| `innerHTML = data` | XSS | Use `textContent` or sanitize |
| `eval(input)` | RCE | Never eval untrusted data |
| `query + input` | SQLi | Parameterized queries |
| `password = "..."` | Leak | Environment variable |

**Suppress if:** Data from trusted internal source (not user input). Document why.

### 3. Async/Await
Silent data loss, race conditions.

```javascript
// SILENT FAILURE
saveUserData(user);  // Missing await

// FIXED
await saveUserData(user);
```

**Suppress if:** Intentional fire-and-forget (logging, analytics). Document: `// ubs:ignore — fire-and-forget, failure acceptable`

### 4. Memory Leaks
Event listeners, timers, subscriptions without cleanup.

```javascript
// LEAK
useEffect(() => {
  window.addEventListener('resize', handler);
}, []);

// FIXED
useEffect(() => {
  window.addEventListener('resize', handler);
  return () => window.removeEventListener('resize', handler);
}, []);
```

**Suppress if:** Global singleton that lives for app lifetime.

### 5. Type Coercion
`==` vs `===`, parseInt without radix, NaN comparisons.

```javascript
// BUG — "" == 0 is true
if (value == 0) { }

// FIXED
if (value === 0) { }
```

**Suppress if:** `== null` to catch both null and undefined (document intent).

---

## Tier 2: Fix Before Merge

### 6. Division Safety
Crash on edge case data.

```python
# CRASH
average = total / len(items)

# FIXED
average = total / len(items) if items else 0
```

### 7. Resource Lifecycle
Memory/handle exhaustion over time.

```python
# LEAK
f = open('data.txt')
data = f.read()

# FIXED
with open('data.txt') as f:
    data = f.read()
```

### 8. Error Handling
Debugging nightmare when things go wrong.

```javascript
// SILENT FAILURE
try { riskyOp(); } catch (e) { }

// FIXED
try { riskyOp(); } catch (e) {
  logger.error('Operation failed', { error: e });
  throw e;
}
```

**Suppress if:** Intentional suppression (cleanup that shouldn't fail main op). Document why.

### 9. Promise Chains
Unhandled rejections.

```javascript
// UNHANDLED
fetch('/api').then(r => r.json()).then(process);

// FIXED
fetch('/api').then(r => r.json()).then(process).catch(handleError);
```

### 10. Array Mutations
Skipped elements, corrupted state.

```javascript
// BUG — skips elements
for (let i = 0; i < arr.length; i++) {
  if (shouldRemove(arr[i])) arr.splice(i, 1);
}

// FIXED
arr = arr.filter(item => !shouldRemove(item));
```

---

## Tier 3: Address or Justify

### 11. Debug Code
`console.log`, `print()`, `debugger`

**Action:** Remove before merge. Use `--skip=11` during active development.

### 12. TODO Markers
TODO, FIXME, HACK, XXX

**Action:** Complete it, create tracking issue, or remove if obsolete.

### 13. Type Safety (`any`)
Defeats type checking.

```typescript
// DEFEATS CHECKING
function process(data: any) { }

// BETTER
function process(data: unknown) {
  if (isValidData(data)) { /* now typed */ }
}
```

**Suppress if:** External API responses, legacy migration (document timeline).

### 14. Readability
Complex ternaries, deep nesting (>4 levels).

**Action:** Refactor for clarity. Don't block merge for style alone.

---

## When to Suppress

**Use `ubs:ignore` when ALL true:**
1. You've verified the code is safe
2. UBS can't see the safety guarantee (cross-file, runtime, API contract)
3. Comment explains WHY it's safe

**Never suppress when:**
- You're not sure if it's safe
- You just want the scan to pass
- You don't understand the code

---

## Language Patterns

See [FALSE-POSITIVES.md](FALSE-POSITIVES.md) for detailed language-specific patterns:
- JS/TS: Optional chaining, React hooks, Promise.all
- Python: Context managers, exception re-raising, binary mode
- Go: Interface nil, defer in loops, goroutine lifetime
- Rust: unwrap() in tests/CLI, safe unsafe wrappers
- Java: Framework-managed resources, checked exceptions
