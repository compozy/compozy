package loop

import (
	"context"
	"fmt"
	"strings"
)

// hydrateGenerationOutputs replaces content-addressed output refs with the payloads they
// stand for, in place.
//
// Outputs at or below LoopOutputInlineLimitBytes are persisted inline, so OutputRef already
// carries the value; larger ones are externalized to the output blob store and OutputRef
// carries a "sha256:…" ref instead. Everything downstream — runtimeNamespace, fan-out
// materialization, gate criteria, action params — reads OutputRef as the value, so an
// unresolved ref silently degrades a node's output to a hash string. Resolving refs here
// keeps externalization a storage detail rather than something every consumer must know
// about.
func hydrateGenerationOutputs(
	ctx context.Context,
	reader GenerationOutputReader,
	outputs []GenerationOutput,
) error {
	if reader == nil {
		return nil
	}
	// One blob can back several outputs (a fan-out branch reusing its producer's payload,
	// a reattempted generation), so resolve each ref once.
	resolved := make(map[string]string)
	for idx := range outputs {
		ref := strings.TrimSpace(outputs[idx].OutputRef)
		if !OutputRefLooksContentAddressed(ref) {
			continue
		}
		payload, seen := resolved[ref]
		if !seen {
			raw, err := reader.GetGenerationOutputPayload(ctx, ref)
			if err != nil {
				return fmt.Errorf("loop: hydrate generation output %q: %w", ref, err)
			}
			payload = string(raw)
			resolved[ref] = payload
		}
		outputs[idx].OutputRef = payload
	}
	return nil
}
