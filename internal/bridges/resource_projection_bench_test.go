package bridges

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/compozy/agh/internal/resources"
)

func BenchmarkBuildResourceState(b *testing.B) {
	cases := []struct {
		name       string
		size       int
		changeLast bool
	}{
		{name: "Noop100", size: 100},
		{name: "Changed100", size: 100, changeLast: true},
		{name: "Noop1000", size: 1000},
		{name: "Changed1000", size: 1000, changeLast: true},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()

			now := time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
			instances := benchmarkBridgeProjectionInstances(tc.size, now)
			records := benchmarkBridgeProjectionRecords(instances, now)
			if tc.changeLast && len(records) > 0 {
				last := len(records) - 1
				records[last].Spec.DisplayName += " Updated"
			}
			store := benchmarkResourceProjectionStore{instances: instances}
			ctx := context.Background()

			for b.Loop() {
				plan, err := BuildResourceState(ctx, store, records, func() time.Time { return now })
				if err != nil {
					b.Fatalf("BuildResourceState() error = %v", err)
				}
				if tc.changeLast && plan.OperationCount() != 1 {
					b.Fatalf("OperationCount() = %d, want 1", plan.OperationCount())
				}
				if !tc.changeLast && plan.OperationCount() != 0 {
					b.Fatalf("OperationCount() = %d, want 0", plan.OperationCount())
				}
			}
		})
	}
}

type benchmarkResourceProjectionStore struct {
	instances []BridgeInstance
}

func (s benchmarkResourceProjectionStore) ListBridgeInstances(context.Context) ([]BridgeInstance, error) {
	return cloneBridgeInstances(s.instances), nil
}

func (s benchmarkResourceProjectionStore) ReplaceBridgeInstances(
	context.Context,
	[]BridgeInstance,
) error {
	return nil
}

func benchmarkBridgeProjectionInstances(size int, now time.Time) []BridgeInstance {
	instances := make([]BridgeInstance, 0, size)
	for idx := range size {
		instances = append(instances, BridgeInstance{
			ID:               fmt.Sprintf("brg-bench-%04d", idx),
			Scope:            ScopeGlobal,
			Platform:         "telegram",
			ExtensionName:    "ext-telegram",
			DisplayName:      fmt.Sprintf("Bridge %04d", idx),
			Source:           BridgeInstanceSourceDynamic,
			Enabled:          true,
			Status:           BridgeStatusReady,
			DMPolicy:         BridgeDMPolicyOpen,
			RoutingPolicy:    RoutingPolicy{IncludePeer: true},
			ProviderConfig:   []byte(`{"tenant":"acme"}`),
			DeliveryDefaults: []byte(`{"peer_id":"peer-1","mode":"reply"}`),
			CreatedAt:        now.Add(-time.Hour),
			UpdatedAt:        now,
		})
	}
	return instances
}

func benchmarkBridgeProjectionRecords(
	instances []BridgeInstance,
	now time.Time,
) []resources.Record[BridgeInstanceSpec] {
	records := make([]resources.Record[BridgeInstanceSpec], 0, len(instances))
	for idx, instance := range instances {
		records = append(records, resources.Record[BridgeInstanceSpec]{
			Kind:      BridgeInstanceResourceKind,
			ID:        instance.ID,
			Version:   int64(idx + 1),
			Scope:     ResourceScopeForBridge(instance.Scope, instance.WorkspaceID),
			Spec:      BridgeInstanceSpecFromInstance(instance),
			CreatedAt: instance.CreatedAt,
			UpdatedAt: now,
		})
	}
	return records
}
