# Issue 5 - Review Thread Comment

**File:** `engine/tool/inline/manager.go:138`
**Date:** 2025-11-01 01:57:01 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Body

_⚠️ Potential issue_ | _🔴 Critical_

**Fix Start retry semantics**

If the first call to `Start` returns an error, `sync.Once` prevents the initialization block from running again, yet the next caller still receives `nil` because `startErr` is reinitialised on every invocation. That leaves the manager inactive while reporting success. Persist the initialization error (and clear it on success) so subsequent callers either see the original failure or the successful state. One way to do that:

```diff
 type Manager struct {
 	opts           Options
 	inlineDir      string
 	entrypointPath string

 	mu             sync.Mutex
 	modules        map[string]moduleState
 	entrypointHash string

 	startOnce sync.Once
 	closeOnce sync.Once
+	startErr  error

 	syncCh chan struct{}
 	cancel context.CancelFunc
 	wg     sync.WaitGroup
 }

 func (m *Manager) Start(ctx context.Context) error {
-	var startErr error
 	m.startOnce.Do(func() {
 		if ctx == nil {
-			startErr = fmt.Errorf("context is required")
+			m.startErr = fmt.Errorf("context is required")
 			return
 		}
 		if err := os.MkdirAll(m.inlineDir, defaultDirPerm); err != nil {
-			startErr = fmt.Errorf("ensure inline directory: %w", err)
+			m.startErr = fmt.Errorf("ensure inline directory: %w", err)
 			return
 		}
@@
-		if err := m.Sync(ctx); err != nil {
-			startErr = err
+		if err := m.Sync(ctx); err != nil {
+			m.startErr = err
 			cancel()
 			m.wg.Wait()
 			return
 		}
@@
-		events, err := m.opts.Store.Watch(syncCtx, m.opts.ProjectName, resources.ResourceTool)
-		if err != nil {
-			startErr = fmt.Errorf("watch tool resources: %w", err)
+		events, err := m.opts.Store.Watch(syncCtx, m.opts.ProjectName, resources.ResourceTool)
+		if err != nil {
+			m.startErr = fmt.Errorf("watch tool resources: %w", err)
 			cancel()
 			m.wg.Wait()
 			return
 		}
+		m.startErr = nil
@@
-	})
-	return startErr
+	})
+	return m.startErr
 }
```

<!-- suggestion_start -->

<details>
<summary>📝 Committable suggestion</summary>

> ‼️ **IMPORTANT**
> Carefully review the code before committing. Ensure that it accurately replaces the highlighted code, contains no missing lines, and has no issues with indentation. Thoroughly test & benchmark the code to ensure it meets the requirements.

```suggestion
type Manager struct {
	opts           Options
	inlineDir      string
	entrypointPath string

	mu             sync.Mutex
	modules        map[string]moduleState
	entrypointHash string

	startOnce sync.Once
	closeOnce sync.Once
	startErr  error

	syncCh chan struct{}
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

func (m *Manager) Start(ctx context.Context) error {
	m.startOnce.Do(func() {
		if ctx == nil {
			m.startErr = fmt.Errorf("context is required")
			return
		}
		if err := os.MkdirAll(m.inlineDir, defaultDirPerm); err != nil {
			m.startErr = fmt.Errorf("ensure inline directory: %w", err)
			return
		}
		syncCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		m.cancel = cancel
		m.syncCh = make(chan struct{}, 1)
		m.wg.Add(1)
		go m.runSyncLoop(syncCtx)
		if err := m.Sync(ctx); err != nil {
			m.startErr = err
			cancel()
			m.wg.Wait()
			return
		}
		events, err := m.opts.Store.Watch(syncCtx, m.opts.ProjectName, resources.ResourceTool)
		if err != nil {
			m.startErr = fmt.Errorf("watch tool resources: %w", err)
			cancel()
			m.wg.Wait()
			return
		}
		m.wg.Add(1)
		go m.runWatcher(syncCtx, events)
		m.startErr = nil
	})
	return m.startErr
}
```

</details>

<!-- suggestion_end -->

<!-- fingerprinting:phantom:medusa:sabertoothed -->

<!-- This is an auto-generated comment by CodeRabbit -->

## Resolve

Thread ID: `PRRT_kwDOOlCPts5gLa2T`

```bash
gh api graphql -f query='mutation($id:ID!){resolveReviewThread(input:{threadId:$id}){thread{isResolved}}}' -F id=PRRT_kwDOOlCPts5gLa2T
```

---
*Generated from PR review - CodeRabbit AI*
