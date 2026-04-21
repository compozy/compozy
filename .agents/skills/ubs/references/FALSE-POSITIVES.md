# False Positive Patterns

## Quick Lookup

| Pattern | Why It's FP | Fix |
|---------|-------------|-----|
| Guard clause before access | UBS misses early return | `// ubs:ignore — guarded above` |
| Caller validates | Cross-file analysis limit | `// ubs:ignore — caller validates` |
| Type system guarantees | UBS doesn't understand type | Non-null assertion or runtime check |
| Test code testing errors | Intentional bad path | `--skip=2` for security in tests |
| Dead code / feature flags | Never executes | Remove dead code |

---

## Universal Patterns

### Guard Clause Before Access

```javascript
// UBS flags user.name but misses the guard
function getName(user) {
  if (!user) return 'Anonymous';
  return user.name;  // Actually safe
}
// ubs:ignore — guarded by early return above
```

### Validation in Caller

```typescript
// UBS flags user.id — can't see call site
function processUser(user: User) {
  saveToDb(user.id);  // Flagged as potentially null
}

// Caller guarantees non-null
const user = await getUser(id);
if (!user) throw new Error('Not found');
processUser(user);  // Safe
// ubs:ignore — caller validates non-null
```

### Type System Guarantees

```typescript
const value = map.get(key);  // Flagged as possibly undefined
value.process();

// If Map<string, NonNullableValue> and key was just inserted
// Add runtime check or non-null assertion with explanation
```

### Test Code

```python
# UBS flags eval() in security test
def test_eval_injection_blocked():
    with pytest.raises(SecurityError):
        process_input("__import__('os').system('rm -rf /')")
# --skip=2 for tests, or: ubs:ignore — test verifying security
```

### Dead Code / Feature Flags

```javascript
if (FEATURE_FLAGS.legacyRenderer) {
  element.innerHTML = content;  // Flagged but legacyRenderer is always false
}
// Remove dead code or document flag state
```

---

## JavaScript/TypeScript

### Optional Chaining

```typescript
// FALSE POSITIVE — ?. handles null
const name = user?.profile?.name;
// Optional chaining IS the guard
```

### Nullish Coalescing

```typescript
// FALSE POSITIVE — ?? handles undefined/null
const value = config.timeout ?? 5000;
```

### React Hooks

```tsx
const [user, setUser] = useState<User | null>(null);
useEffect(() => { fetchUser().then(setUser); }, []);
return <div>{user.name}</div>;  // Flagged

// If component only renders after loading, add loading guard
```

### Promise.all with Proper Handling

```javascript
// FALSE POSITIVE if results are properly destructured
const [users, posts] = await Promise.all([
  fetchUsers(),
  fetchPosts()
]);
// UBS may flag the array access pattern
```

### Intentional Fire-and-Forget

```javascript
// FALSE POSITIVE for analytics
analytics.track('page_view');  // No await
// ubs:ignore — fire-and-forget, failure acceptable
```

---

## Python

### Context Manager in Wrapper

```python
# FALSE POSITIVE — file closed by wrapper
f = open(path)  # Flagged
return ConfigParser(f)  # ConfigParser closes on __del__

# Better: make explicit
with open(path) as f:
    return ConfigParser(f.read())
```

### Exception Re-raised

```python
# FALSE POSITIVE — not swallowed
try:
    risky_operation()
except SpecificError as e:
    logger.error(f"Failed: {e}")
    raise  # Re-raised, not swallowed
```

### Binary Mode

```python
# FALSE POSITIVE — encoding not needed
with open('image.png', 'rb') as f:  # UBS may want encoding=
    data = f.read()  # Binary mode, no encoding needed
```

### eval() with Literal

```python
# FALSE POSITIVE — literal expression, not user input
result = eval('2 + 2')  # Flagged but safe

# Better: use literal_eval for safety
from ast import literal_eval
result = literal_eval('2 + 2')
```

---

## Go

### Interface Nil vs Concrete Nil

```go
// TRICKY — interface nil check
var p *MyError = nil
var err error = p  // err is NOT nil (concrete nil in interface)
if err != nil { }  // May be flagged but behavior is subtle
```

### Defer in Loop

```go
// SOMETIMES FP
for _, file := range files {
    f, _ := os.Open(file)
    defer f.Close()  // Defers until function exit
}
// Usually better: wrap in func() { defer f.Close(); process() }()
```

### Error Ignored with Comment

```go
// FALSE POSITIVE if intentional
_ = writer.Flush()  // Flagged

// ubs:ignore — best-effort flush, error logged elsewhere
```

### Goroutine Bounded by main()

```go
func main() {
    go backgroundTask()  // Flagged — no stop mechanism
    // For simple CLIs, program exit stops it
}
// ubs:ignore — CLI lifetime bounded
```

---

## Rust

### unwrap() in Tests

```rust
#[test]
fn test_parse() {
    let result = parse("valid").unwrap();  // Flagged
    assert_eq!(result, expected);
    // Tests SHOULD panic on unexpected None
}
```

### unwrap() in CLI Main

```rust
fn main() {
    let config = Config::load().unwrap();  // Flagged
    // For CLIs, panic with backtrace IS the error UX
}
```

### Safe Unsafe Wrappers

```rust
pub fn safe_wrapper() -> Result<()> {
    unsafe { ffi::well_tested_c_function(); }  // Flagged
    Ok(())
}
// ubs:ignore — FFI wrapper, C function validated safe
```

### spawn() for Background Tasks

```rust
tokio::spawn(async move {
    cleanup_old_files().await;  // Flagged — no join
});
// ubs:ignore — fire-and-forget cleanup, ok if dropped
```

---

## Java

### Framework-Managed Resources

```java
@Autowired
DataSource dataSource;  // Flagged as unmanaged
// Spring manages the connection pool lifecycle
```

### Wrapped and Re-thrown

```java
try {
    Files.readString(path);
} catch (IOException e) {
    throw new RuntimeException(e);  // Flagged as "swallowed"
    // Wrapped and re-thrown, not swallowed
}
```

### AutoCloseable Returned to Caller

```java
public InputStream getResource() {
    return new FileInputStream(file);  // Flagged
    // Caller responsibility — document in Javadoc
}
```

---

## The Decision Table

| Situation | Likely FP | Likely Real |
|-----------|-----------|-------------|
| Guard clause exists | FP | — |
| Caller validates | FP | — |
| Test code | FP | — |
| Type system guarantees | FP | — |
| "It works in practice" | — | **REAL** — luck isn't safety |
| "Always been this way" | — | **REAL** — tech debt |
| "Data is always valid" | — | **REAL** — data changes |
| "Users won't do that" | — | **REAL** — users do everything |

---

## The Golden Rule

> If you need to think hard about whether it's a false positive, **treat it as real and add the guard**.
>
> The cost of a redundant check is nearly zero.
> The cost of a missed bug is high.
