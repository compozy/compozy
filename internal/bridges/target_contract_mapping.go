package bridges

import (
	"slices"

	bridgecontract "github.com/compozy/agh/internal/bridges/contract"
)

func bridgeTargetSnapshotRequestToContract(
	request BridgeTargetSnapshotRequest,
) bridgecontract.BridgeTargetSnapshotRequest {
	return bridgecontract.BridgeTargetSnapshotRequest{BridgeInstanceID: request.BridgeInstanceID}
}

func bridgeTargetSnapshotToContract(snapshot BridgeTargetSnapshot) bridgecontract.BridgeTargetSnapshot {
	return bridgecontract.BridgeTargetSnapshot{
		CanonicalRoute: snapshot.CanonicalRoute,
		DisplayName:    snapshot.DisplayName,
		TargetType:     bridgecontract.BridgeTargetType(snapshot.TargetType),
		Qualifier:      snapshot.Qualifier,
		Capabilities:   slices.Clone(snapshot.Capabilities),
		LastSeenAt:     snapshot.LastSeenAt,
	}
}
