package loop

import (
	"strings"

	"github.com/compozy/compozy/internal/hooks"
	"github.com/compozy/compozy/internal/loop/dsl"
)

func referencedWatchEventsContracts(
	definition dsl.Definition,
	available map[hooks.HookEvent]WatchEventsContract,
) map[hooks.HookEvent]WatchEventsContract {
	contracts := map[hooks.HookEvent]WatchEventsContract{}
	collectGraphWatchEventsContracts(definition.Graph, available, contracts)
	return contracts
}

func collectGraphWatchEventsContracts(
	graph dsl.Graph,
	available map[hooks.HookEvent]WatchEventsContract,
	contracts map[hooks.HookEvent]WatchEventsContract,
) {
	for _, node := range graph.Nodes {
		for _, subscription := range nodeWatchEventSubscriptions(node) {
			kind := hooks.HookEvent(strings.TrimSpace(subscription.Kind))
			if contract, ok := available[kind]; ok {
				contracts[kind] = cloneWatchEventsContract(contract)
			}
		}
		if node.Body != nil {
			collectGraphWatchEventsContracts(*node.Body, available, contracts)
		}
	}
}

func nodeWatchEventSubscriptions(node dsl.Node) []dsl.EventSubscription {
	if node.Class != dsl.NodeClassControl || dsl.ControlKind(node.Kind) != dsl.ControlWait {
		return node.Events
	}
	var params dsl.WaitParams
	if err := node.Params.Decode(&params); err != nil || params.Event == nil {
		return nil
	}
	return []dsl.EventSubscription{*params.Event}
}

func cloneWatchEventsContracts(
	contracts map[hooks.HookEvent]WatchEventsContract,
) map[hooks.HookEvent]WatchEventsContract {
	cloned := make(map[hooks.HookEvent]WatchEventsContract, len(contracts))
	for kind, contract := range contracts {
		cloned[kind] = cloneWatchEventsContract(contract)
	}
	return cloned
}
