# Duplicate from Comment 5

**File:** `sdk/compozy/registration_errors_test.go`
**Date:** 2025-11-01 12:25:25 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Resolution

- Wrapped the persist failure scenario in a `t.Run` block with `t.Parallel()` to align with the suite's Should-style expectations.

## Details

<details>
<summary>sdk/compozy/registration_errors_test.go (1)</summary><blockquote>

`206-216`: **Wrap this test in a Should-style subtest**

This package mandates `t.Run("Should ...")` subtests for every behavior. Please wrap the body in a `t.Run` block so the test complies. As per coding guidelines.

```diff
 func TestRegisterProjectResetsStateOnPersistFailure(t *testing.T) {
-	store := newResourceStoreStub()
-	store.putErr = errors.New("persist failure")
-	engine := &Engine{ctx: t.Context(), resourceStore: store}
-	cfg := &engineproject.Config{Name: "helios"}
-	err := engine.registerProject(cfg, registrationSourceProgrammatic)
-	require.Error(t, err)
-	engine.stateMu.RLock()
-	defer engine.stateMu.RUnlock()
-	assert.Nil(t, engine.project)
+	t.Run("Should reset project state on persist failure", func(t *testing.T) {
+		store := newResourceStoreStub()
+		store.putErr = errors.New("persist failure")
+		engine := &Engine{ctx: t.Context(), resourceStore: store}
+		cfg := &engineproject.Config{Name: "helios"}
+		err := engine.registerProject(cfg, registrationSourceProgrammatic)
+		require.Error(t, err)
+		engine.stateMu.RLock()
+		defer engine.stateMu.RUnlock()
+		assert.Nil(t, engine.project)
+	})
 }
```

</blockquote></details>
