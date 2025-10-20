# Duplicate comments for `engine/knowledge/uc/loader.go`

## Duplicate from Comment 4

**File:** `engine/knowledge/uc/loader.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
<summary>engine/knowledge/uc/loader.go (2)</summary><blockquote>

`66-86`: **Validate embedderID is non-empty before store lookup.**

The trimmed embedderID should be validated before calling store.Get to avoid querying with an empty ID, which can lead to confusing errors.




Apply this diff to add validation:

```diff
 func loadEmbedderConfig(
 	ctx context.Context,
 	store resources.ResourceStore,
 	projectID string,
 	embedderID string,
 ) (*knowledge.EmbedderConfig, error) {
+	id := strings.TrimSpace(embedderID)
+	if id == "" {
+		return nil, fmt.Errorf("load embedder: id is required")
+	}
 	embKey := resources.ResourceKey{
 		Project: projectID,
 		Type:    resources.ResourceEmbedder,
-		ID:      strings.TrimSpace(embedderID),
+		ID:      id,
 	}
```

Based on learnings

---

`88-108`: **Validate vectorID is non-empty before store lookup.**

The trimmed vectorID should be validated before calling store.Get to avoid querying with an empty ID, which can lead to confusing errors.




Apply this diff to add validation:

```diff
 func loadVectorDBConfig(
 	ctx context.Context,
 	store resources.ResourceStore,
 	projectID string,
 	vectorID string,
 ) (*knowledge.VectorDBConfig, error) {
+	id := strings.TrimSpace(vectorID)
+	if id == "" {
+		return nil, fmt.Errorf("load vector_db: id is required")
+	}
 	vecKey := resources.ResourceKey{
 		Project: projectID,
 		Type:    resources.ResourceVectorDB,
-		ID:      strings.TrimSpace(vectorID),
+		ID:      id,
 	}
```

Based on learnings

</blockquote></details>
