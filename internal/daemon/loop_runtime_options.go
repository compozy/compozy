package daemon

import (
	"slices"

	"github.com/compozy/compozy/internal/acp"
	looppkg "github.com/compozy/compozy/internal/loop"
	"github.com/compozy/compozy/internal/loop/dsl"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
)

func loopACPOptionsForSession(
	options []looppkg.ACPOptionSelection,
) []acp.SessionConfigOptionSelection {
	return session.ACPOptionSelectionsFromConfig(options)
}

func cloneLoopACPOptions(options []looppkg.ACPOptionSelection) []looppkg.ACPOptionSelection {
	return dsl.CloneACPOptionSelections(options)
}

func loopACPOptionsFromSession(
	options []acp.SessionConfigOptionSelection,
) []looppkg.ACPOptionSelection {
	return session.ConfigACPOptionSelectionsFromACP(options)
}

func loopACPOptionsFromStore(
	options []store.SessionACPOptionSelection,
) []looppkg.ACPOptionSelection {
	return loopACPOptionsFromSession(session.ACPOptionSelectionsFromStore(options))
}

func loopACPOptionsMatchStore(
	requested []looppkg.ACPOptionSelection,
	persisted []store.SessionACPOptionSelection,
) bool {
	left := dsl.CanonicalACPOptionSelections(requested)
	right := dsl.CanonicalACPOptionSelections(loopACPOptionsFromStore(persisted))
	return slices.EqualFunc(left, right, func(a, b looppkg.ACPOptionSelection) bool {
		if a.ID != b.ID || a.ValueID != b.ValueID || (a.BoolValue == nil) != (b.BoolValue == nil) {
			return false
		}
		return a.BoolValue == nil || *a.BoolValue == *b.BoolValue
	})
}
