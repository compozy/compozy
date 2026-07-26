package settings

import "github.com/compozy/agh/internal/config/lifecycle"

type collectionMutationOperation string

const (
	collectionMutationDelete collectionMutationOperation = "delete"
	collectionMutationPut    collectionMutationOperation = "put"
)

func applyCollectionLifecycle(
	result MutationResult,
	collection CollectionName,
	operation collectionMutationOperation,
	existedBefore bool,
) MutationResult {
	configLifecycle := lifecycle.RestartRequired
	switch collection {
	case CollectionProviders:
		if operation == collectionMutationPut && result.Lifecycle == lifecycle.Live {
			configLifecycle = lifecycle.Live
		} else if operation == collectionMutationPut && !existedBefore {
			configLifecycle = lifecycle.LiveAdd
		}
		if operation == collectionMutationDelete {
			configLifecycle = lifecycle.LiveRemoveIfUnused
		}
	case CollectionMCPServers:
		if operation == collectionMutationPut && !existedBefore {
			configLifecycle = lifecycle.LiveAdd
		}
		if operation == collectionMutationDelete {
			configLifecycle = lifecycle.LiveRemoveIfUnused
		}
	case CollectionSandboxes:
		configLifecycle = lifecycle.SessionRebind
	case CollectionHooks:
		configLifecycle = lifecycle.RestartRequired
	}
	result.Lifecycle = configLifecycle
	result.DiffClass = lifecycle.DiffClass(configLifecycle)
	classification := classificationFromLifecycle(configLifecycle, lifecycle.DiffClass(configLifecycle))
	result.Behavior = classification.Behavior
	result.Applied = classification.Applied
	result.RestartRequired = classification.RestartRequired
	result.RestartScope = classification.RestartScope
	return result
}
