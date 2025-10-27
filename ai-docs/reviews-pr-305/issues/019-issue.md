# Duplicate from Comment 3

**File:** `scripts/markdown/check.go`
**Date:** 2025-10-27 13:58:54 America/Sao_Paulo
**Status:** - [x] RESOLVED

## Details

<details>
<summary>scripts/markdown/check.go (1)</summary><blockquote>

`1868-1877`: **Use the named constant instead of literal value.**

Line 1874 uses the literal `5 * time.Second` instead of the `processTerminationGracePeriod` constant defined at the top of the file. For consistency and maintainability, use the constant.


Apply this diff:

```diff
 	select {
 	case <-cmdDone:
 		if !useUI {
 			fmt.Fprintf(os.Stderr, "Job %d terminated gracefully after timeout\n", index+1)
 		}
-	case <-time.After(5 * time.Second):
+	case <-time.After(processTerminationGracePeriod):
 		forceKillProcess(cmd, index, useUI)
 	}
```

</blockquote></details>
