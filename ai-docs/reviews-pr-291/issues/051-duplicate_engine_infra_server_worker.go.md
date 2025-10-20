# Duplicate comments for `engine/infra/server/worker.go`

## Duplicate from Comment 4

**File:** `engine/infra/server/worker.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
<summary>engine/infra/server/worker.go (2)</summary><blockquote>

`202-212`: **Replace magic number time.Second with a named constant.**

Define a clear, typed default and use it in the fallback branch. This was previously flagged and remains unresolved.  As per coding guidelines.

```diff
@@
-const maxScheduleRetryAttemptsCap = 1_000_000
+const maxScheduleRetryAttemptsCap = 1_000_000
+const defaultScheduleRetryBaseDelay time.Duration = time.Second
@@
 func scheduleRetryBaseDelay(cfg *config.Config) time.Duration {
   base := cfg.Server.Timeouts.ScheduleRetryBaseDelay
   if secs := cfg.Server.Timeouts.ScheduleRetryBackoffSeconds; secs > 0 {
     base = time.Duration(secs) * time.Second
   }
   if base <= 0 {
-    return time.Second
+    return defaultScheduleRetryBaseDelay
   }
   return base
 }
```

---

`186-187`: **Do not pass logger or config; retrieve both from context inside the handler.**

This repeats prior feedback about the logger parameter and extends it to config. Runtime code must use logger.FromContext(ctx) and config.FromContext(ctx). Update the call site and signature accordingly.  As per coding guidelines.

```diff
@@
-  s.handleScheduleReconciliationFailure(err, log, startTime, cfg)
+  s.handleScheduleReconciliationFailure(err, startTime)
```

```diff
@@
-func (s *Server) handleScheduleReconciliationFailure(
-  err error,
-  log logger.Logger,
-  start time.Time,
-  cfg *config.Config,
-) {
+func (s *Server) handleScheduleReconciliationFailure(
+  err error,
+  start time.Time,
+) {
   if err == nil {
     return
   }
-  if s.ctx.Err() == context.Canceled {
-    log.Info("Schedule reconciliation canceled during server shutdown")
-    return
-  }
+  log := logger.FromContext(s.ctx)
+  cfg := config.FromContext(s.ctx)
+  if s.ctx.Err() == context.Canceled {
+    log.Info("Schedule reconciliation canceled during server shutdown")
+    return
+  }
   finalErr := fmt.Errorf("schedule reconciliation failed after maximum retries: %w", err)
   s.reconciliationState.setError(finalErr)
   log.Error("Schedule reconciliation exhausted retries",
     "error", err,
     "duration", time.Since(start),
     "max_duration", cfg.Server.Timeouts.ScheduleRetryMaxDuration)
 }
```


Also applies to: 214-234

</blockquote></details>
