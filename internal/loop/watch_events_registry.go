package loop

import (
	"slices"

	"github.com/compozy/agh/internal/hooks"
)

// SupportedWatchEvents returns the closed watch-events registry for shipped phases.
func SupportedWatchEvents() map[hooks.HookEvent]WatchEventsContract {
	rows := make(
		[]WatchEventsContract,
		0,
		len(phaseAWatchEvents)+len(phaseBWatchEvents)+len(phaseCWatchEvents),
	)
	rows = append(rows, phaseAWatchEvents...)
	rows = append(rows, phaseBWatchEvents...)
	rows = append(rows, phaseCWatchEvents...)
	contracts := make(map[hooks.HookEvent]WatchEventsContract, len(rows))
	for _, contract := range rows {
		contracts[contract.Kind] = cloneWatchEventsContract(contract)
	}
	return contracts
}

func cloneWatchEventsContract(contract WatchEventsContract) WatchEventsContract {
	contract.LedgerTypes = slices.Clone(contract.LedgerTypes)
	contract.PayloadFields = slices.Clone(contract.PayloadFields)
	contract.RequiredVars = slices.Clone(contract.RequiredVars)
	return contract
}

func sortedSupportedWatchEventKinds() []string {
	contracts := SupportedWatchEvents()
	kinds := make([]string, 0, len(contracts))
	for kind := range contracts {
		kinds = append(kinds, string(kind))
	}
	slices.Sort(kinds)
	return kinds
}
