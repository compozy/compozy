# Duplicate comments for `cli/helpers/io.go`

## Duplicate from Comment 4

**File:** `cli/helpers/io.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
<summary>cli/helpers/io.go (2)</summary><blockquote>

`313-323`: **Reset lastModTime when file is missing.**

When `processFileChange` returns `errFileMissing` (line 315), `lastModTime` is not reset. Without resetting, a re-created file whose modtime ≤ `lastModTime` may never trigger the callback.



Apply this diff:

```diff
        updated, modTime, err := processFileChange(ctx, path, callback, lastModTime)
        if err != nil {
          if errors.Is(err, errFileMissing) {
+           lastModTime = time.Time{}
            continue
          }
          return err
        }
```

---

`304-307`: **Initial-missing file should not prevent watch startup.**

The code returns an error if the file doesn't exist initially (line 306), preventing the watcher from starting. Per past review feedback, allow the watch to start even if the file is absent, so it can detect file creation.



Apply this diff:

```diff
  lastModTime, err := fileModTime(path)
  if err != nil {
-   return err
+   if errors.Is(err, errFileMissing) {
+     logger.FromContext(ctx).Debug("file not found; will wait for creation", "file", path)
+     lastModTime = time.Time{}
+   } else {
+     return err
+   }
  }
```

</blockquote></details>
