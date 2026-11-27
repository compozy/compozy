package core

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/compozy/compozy/internal/admission"
	"github.com/compozy/compozy/internal/api/contract"
)

func (h *BaseHandlers) GetHostResources(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, h.hostResourceSample())
}

func (h *BaseHandlers) hostResourceSample() contract.HostResourcesResponse {
	var snapshot admission.LoadSnapshot
	if h.HostLoad != nil {
		snapshot = h.HostLoad.Snapshot()
	}
	return hostResources(snapshot)
}

func hostResources(snapshot admission.LoadSnapshot) contract.HostResourcesResponse {
	resources := contract.HostResourcesResponse{CPUCount: snapshot.LogicalCPUs, SampledAt: snapshot.SampledAt}
	if snapshot.Supported {
		resources.CPUUtil = new(min(1, snapshot.NormalizedLoad))
	}
	if snapshot.Memory.Total > 0 {
		resources.MemTotal = new(snapshot.Memory.Total)
		resources.MemAvailable = new(snapshot.Memory.Available)
	}
	return resources
}
