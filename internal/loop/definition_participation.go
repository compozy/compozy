package loop

import (
	"fmt"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/network/participation"
)

func (c *lintContext) lintNetworkParticipation() {
	normalized, err := normalizedDefinitionParticipation(c.def.NetworkParticipation)
	if err != nil {
		c.add("", CodeNetworkParticipationInvalid, "network_participation is invalid: %v", err)
		return
	}
	if definitionParticipationIsLive(normalized) {
		return
	}
	node, found := firstNetworkUsingNode(c.def.Graph)
	if !found {
		return
	}
	c.add(
		node.ID,
		CodeLoopRequiresLive,
		"node %q uses %q and requires live participation",
		node.ID,
		networkNodeCapability(node),
	)
}

func normalizeDefinitionParticipation(def *dsl.Definition) error {
	if def == nil {
		return fmt.Errorf("definition is required")
	}
	normalized, err := normalizedDefinitionParticipation(def.NetworkParticipation)
	if err != nil {
		return err
	}
	extension := *def.DefinitionExtensionState
	if extension.Start != nil {
		extension.Start = append(make([]dsl.StartBinding, 0, len(extension.Start)), extension.Start...)
	}
	extension.NetworkParticipation = normalized
	def.DefinitionExtensionState = &extension
	return nil
}

func normalizedDefinitionParticipation(
	request *participation.Request,
) (*participation.Request, error) {
	if request == nil {
		return nil, nil
	}
	normalized, err := participation.NormalizeIntent(*request)
	if err != nil {
		return nil, err
	}
	if normalized == (participation.Request{}) {
		return nil, nil
	}
	return &normalized, nil
}

func definitionParticipationIsLive(request *participation.Request) bool {
	return request != nil && request.Mode != nil && *request.Mode == participation.ModeLive
}
