# Redis Key Prefix Duplication Fix

## Critical Issue Fixed

The `NewRedisMemoryStore` constructor was incorrectly adding a default "compozy:memory:" prefix when an empty string was passed, causing double-prefixing of keys. This resulted in keys like:

- Expected: `compozy:proj1:memory:user1`
- Actual (buggy): `compozy:memory:compozy:proj1:memory:user1`

## Changes Made

### 1. RedisMemoryStore Constructor (store.go)

- Removed the automatic default prefix assignment when empty string is passed
- Added comprehensive documentation explaining the caller's responsibility
- Constructor now accepts empty prefix without modification

### 2. Key Helper Methods (store.go)

- Updated `fullKey()` to handle empty prefix correctly
- Updated `metadataKey()` to handle empty prefix correctly
- Both methods now return the key unmodified when prefix is empty

### 3. Comprehensive Tests (store_test.go)

- Added `TestRedisMemoryStore_KeyPrefixHandling` with three scenarios:
    - Empty prefix handling (MemoryManager use case)
    - Non-empty prefix handling (custom use cases)
    - Double-prefixing prevention verification

## Backward Compatibility Considerations

### Data Migration Required

Existing data stored with the buggy double-prefix will need migration:

```go
// Example migration approach
func MigrateRedisKeys(ctx context.Context, redisClient cache.RedisInterface) error {
    // Pattern to find double-prefixed keys
    buggyPattern := "compozy:memory:compozy:*"

    // Use SCAN to find all affected keys
    iter := redisClient.Scan(ctx, 0, buggyPattern, 0).Iterator()
    for iter.Next(ctx) {
        oldKey := iter.Val()

        // Extract the correct key by removing the duplicate prefix
        // "compozy:memory:compozy:proj1:memory:user1" -> "compozy:proj1:memory:user1"
        newKey := strings.Replace(oldKey, "compozy:memory:", "", 1)

        // Rename the key
        if err := redisClient.Rename(ctx, oldKey, newKey).Err(); err != nil {
            return fmt.Errorf("failed to rename key %s to %s: %w", oldKey, newKey, err)
        }

        // Also migrate metadata keys
        oldMetaKey := oldKey + ":metadata"
        newMetaKey := newKey + ":metadata"
        if exists := redisClient.Exists(ctx, oldMetaKey).Val(); exists > 0 {
            if err := redisClient.Rename(ctx, oldMetaKey, newMetaKey).Err(); err != nil {
                return fmt.Errorf("failed to rename metadata key %s to %s: %w", oldMetaKey, newMetaKey, err)
            }
        }
    }

    if err := iter.Err(); err != nil {
        return fmt.Errorf("error scanning keys: %w", err)
    }

    return nil
}
```

### Breaking Changes

1. **Custom RedisMemoryStore Usage**: If any code was directly instantiating `RedisMemoryStore` with an empty prefix and relying on the automatic "compozy:memory:" prefix, it will break.

    - **Fix**: Pass "compozy:memory:" explicitly if that prefix is desired

2. **Key Assumptions**: Any code assuming the double-prefix structure will fail to find existing data.
    - **Fix**: Run the migration script above before deploying the fix

### Deployment Strategy

1. **Identify affected systems**: Check Redis for keys matching `compozy:memory:compozy:*` pattern
2. **Test migration**: Run migration in a staging environment first
3. **Deploy with caution**:
    - Option A: Deploy fix + run migration immediately (brief downtime)
    - Option B: Deploy a version that can read both old and new keys, migrate gradually, then remove old key support
4. **Monitor**: Watch for any "key not found" errors after deployment

## Testing the Fix

Run the specific test to verify the fix:

```bash
go test ./engine/memory -run TestRedisMemoryStore_KeyPrefixHandling -v
```

Run all store tests to ensure no regression:

```bash
go test ./engine/memory -run TestRedisMemoryStore -v
```

## Future Improvements

1. Consider adding a version field to Redis keys for easier migrations
2. Add monitoring/alerting for key format inconsistencies
3. Consider using a key builder pattern instead of string concatenation
