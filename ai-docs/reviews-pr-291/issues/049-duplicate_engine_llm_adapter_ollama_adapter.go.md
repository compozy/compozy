# Duplicate comments for `engine/llm/adapter/ollama_adapter.go`

## Duplicate from Comment 4

**File:** `engine/llm/adapter/ollama_adapter.go`
**Date:** 2025-10-20 10:04:06 America/Sao_Paulo
**Status:** - [x] RESOLVED ✓

## Details

<details>
<summary>engine/llm/adapter/ollama_adapter.go (2)</summary><blockquote>

`591-618`: **Add missing FrequencyPenalty and PresencePenalty mappings.**

The `CallOptions` struct defines both `FrequencyPenalty` and `PresencePenalty` fields, and the Ollama Go SDK supports these options, but this function does not map them to the options dictionary.




Apply this diff:

```diff
 func applyOllamaSamplingOptions(options map[string]any, opts *CallOptions) {
 	if opts == nil {
 		return
 	}
 	if opts.TemperatureSet {
 		options["temperature"] = opts.Temperature
 	} else if opts.Temperature > 0 {
 		options["temperature"] = opts.Temperature
 	}
 	if opts.TopP > 0 {
 		options["top_p"] = opts.TopP
 	}
 	if opts.TopK > 0 {
 		options["top_k"] = opts.TopK
 	}
+	if opts.FrequencyPenalty > 0 {
+		options["frequency_penalty"] = opts.FrequencyPenalty
+	}
+	if opts.PresencePenalty > 0 {
+		options["presence_penalty"] = opts.PresencePenalty
+	}
 	if opts.Seed != 0 {
 		options["seed"] = opts.Seed
 	}
 	if opts.MaxTokens > 0 {
 		options["num_predict"] = opts.MaxTokens
 	}
 	if opts.RepetitionPenalty > 0 {
 		options["repeat_penalty"] = opts.RepetitionPenalty
 	}
 	if len(opts.StopWords) > 0 {
 		options["stop"] = opts.StopWords
 	}
 }
```

---

`620-637`: **Cache logger outside the loop for efficiency.**

The function retrieves the logger from context on every iteration (line 627). Cache it once before the loop to avoid repeated context lookups.




Apply this diff:

```diff
 func (a *ollamaAdapter) mergeMetadataOptions(ctx context.Context, options map[string]any, metadata map[string]any) {
+	log := logger.FromContext(ctx)
 	cloned := core.CloneMap(metadata)
 	for key, value := range cloned {
 		if _, exists := options[key]; exists {
 			continue
 		}
 		if !isSupportedOllamaOptionValue(value) {
-			logger.FromContext(ctx).Debug(
+			log.Debug(
 				"Skipping unsupported Ollama option metadata value",
 				"provider", string(a.provider.Provider),
 				"key", key,
 				"type", fmt.Sprintf("%T", value),
 			)
 			continue
 		}
 		options[key] = normalizeOllamaOptionValue(value)
 	}
 }
```

</blockquote></details>
