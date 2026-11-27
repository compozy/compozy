package core

import (
	"testing"
	"time"

	"github.com/compozy/compozy/internal/admission"
)

func TestHostResources(t *testing.T) {
	t.Parallel()

	t.Run("Should clamp supported CPU load and omit unavailable values [UT-108]", func(t *testing.T) {
		t.Parallel()
		sampledAt := time.Now().UTC()
		resources := hostResources(admission.LoadSnapshot{
			Supported: true, LogicalCPUs: 8, NormalizedLoad: 1.7,
			Memory: admission.MemorySnapshot{Total: 512, Available: 256}, SampledAt: sampledAt,
		})
		if resources.CPUCount != 8 || resources.CPUUtil == nil || *resources.CPUUtil != 1 ||
			resources.MemTotal == nil || *resources.MemTotal != 512 ||
			resources.MemAvailable == nil || *resources.MemAvailable != 256 || !resources.SampledAt.Equal(sampledAt) {
			t.Fatalf("host resources = %+v, want eight CPUs at full utilization and 256 of 512 bytes free", resources)
		}
		resources = hostResources(admission.LoadSnapshot{LogicalCPUs: 8})
		if resources.CPUCount != 8 || resources.CPUUtil != nil || resources.MemTotal != nil ||
			resources.MemAvailable != nil {
			t.Fatalf("unsupported host resources = %+v, want nullable utilization and memory", resources)
		}
	})
}
