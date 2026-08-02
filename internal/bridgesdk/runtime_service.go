package bridgesdk

import (
	"context"
	"encoding/json"
	"fmt"

	bridgepkg "github.com/compozy/compozy/internal/bridges/contract"
	"github.com/compozy/compozy/internal/subprocess"
)

func (r *Runtime) handleTargetSnapshots(ctx context.Context, raw json.RawMessage) (any, error) {
	session, err := r.requireServiceSession("bridge target snapshots")
	if err != nil {
		return nil, err
	}

	var request bridgepkg.BridgeTargetSnapshotRequest
	if err := decodeParams(raw, &request); err != nil {
		return nil, err
	}
	if err := request.Validate(); err != nil {
		return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
			runtimeErrorKey: err.Error(),
		})
	}

	targets, err := r.config.TargetSnapshots(ctx, session, request)
	if err != nil {
		return nil, err
	}
	response := bridgepkg.BridgeTargetSnapshotResponse{Targets: targets}
	for index, target := range response.Targets {
		if err := target.Validate(); err != nil {
			return nil, subprocess.NewRPCError(bridgeSDKRPCCodeInvalidParams, "Invalid params", map[string]string{
				runtimeErrorKey: fmt.Sprintf("target %d: %s", index, err.Error()),
			})
		}
	}
	return response, nil
}

func (r *Runtime) handleHealthCheck(ctx context.Context, _ json.RawMessage) (any, error) {
	session, err := r.requireServiceSession("health check")
	if err != nil {
		return nil, err
	}
	if r.config.HealthCheck != nil {
		if err := r.config.HealthCheck(ctx, session); err != nil {
			return nil, err
		}
	}
	return subprocess.HealthCheckResponse{Healthy: true}, nil
}
