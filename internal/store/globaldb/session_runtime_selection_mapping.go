package globaldb

import (
	"strings"

	speedpkg "github.com/compozy/compozy/internal/speed"
	"github.com/compozy/compozy/internal/store"
)

func decodeSelectedRuntime(provider, model, reasoningEffort, speed string) *store.SessionRuntimeSelection {
	if strings.TrimSpace(provider) == "" {
		return nil
	}
	return store.NormalizeSessionRuntimeSelection(&store.SessionRuntimeSelection{
		Provider:        provider,
		Model:           model,
		ReasoningEffort: reasoningEffort,
		Speed:           speedpkg.Speed(strings.TrimSpace(speed)),
	})
}

func selectedRuntimeProvider(selection *store.SessionRuntimeSelection) string {
	return selectedRuntimeValue(selection, func(value *store.SessionRuntimeSelection) string { return value.Provider })
}

func selectedRuntimeModel(selection *store.SessionRuntimeSelection) string {
	return selectedRuntimeValue(selection, func(value *store.SessionRuntimeSelection) string { return value.Model })
}

func selectedRuntimeReasoningEffort(selection *store.SessionRuntimeSelection) string {
	return selectedRuntimeValue(
		selection,
		func(value *store.SessionRuntimeSelection) string { return value.ReasoningEffort },
	)
}

func selectedRuntimeSpeed(selection *store.SessionRuntimeSelection) string {
	return selectedRuntimeValue(
		selection,
		func(value *store.SessionRuntimeSelection) string { return string(value.Speed) },
	)
}

func selectedRuntimeValue(
	selection *store.SessionRuntimeSelection,
	value func(*store.SessionRuntimeSelection) string,
) string {
	normalized := store.NormalizeSessionRuntimeSelection(selection)
	if normalized == nil {
		return ""
	}
	return value(normalized)
}
