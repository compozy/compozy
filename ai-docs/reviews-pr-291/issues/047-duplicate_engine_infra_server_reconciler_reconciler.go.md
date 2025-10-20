# Duplicate comments for `engine/infra/server/reconciler/reconciler.go`

## Duplicate from Comment 4

**File:** `engine/infra/server/reconciler/reconciler.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
<summary>engine/infra/server/reconciler/reconciler.go (1)</summary><blockquote>

`106-107`: **Constant placement issue remains unresolved.**

The constant is declared here but only used in `newReconcilerMetrics` (lines 59–93). For better locality of reference, it should be moved immediately before that function, as noted in the previous review.

</blockquote></details>
