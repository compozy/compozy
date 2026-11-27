package contract

import "time"

const HostResourcesPath = "/api/host/resources"

type HostResourcesResponse struct {
	CPUCount     int       `json:"cpu_count"`
	CPUUtil      *float64  `json:"cpu_util"`
	MemTotal     *uint64   `json:"mem_total"`
	MemAvailable *uint64   `json:"mem_available"`
	SampledAt    time.Time `json:"sampled_at"`
}
