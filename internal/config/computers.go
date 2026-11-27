package config

import "slices"

const DefaultComputerWeight = 50

type ComputersConfig struct {
	LocalRunAgents           bool `toml:"local_run_agents"`
	LoadBalancing            bool `toml:"load_balancing"`
	LocalLoadBalancingWeight int  `toml:"local_load_balancing_weight"`
}

func DefaultComputersConfig() ComputersConfig {
	return ComputersConfig{LocalRunAgents: true, LocalLoadBalancingWeight: DefaultComputerWeight}
}

func IsComputerWeight(weight int) bool {
	return slices.Contains([]int{0, 25, DefaultComputerWeight, 100}, weight)
}

func (c ComputersConfig) Validate() error {
	if !IsComputerWeight(c.LocalLoadBalancingWeight) {
		return ValidationError{
			Path: "computers.local_load_balancing_weight", Message: "choose a weight of 0, 25, 50, or 100",
		}
	}
	return nil
}

type computersOverlay struct {
	LocalRunAgents           *bool `toml:"local_run_agents"`
	LoadBalancing            *bool `toml:"load_balancing"`
	LocalLoadBalancingWeight *int  `toml:"local_load_balancing_weight"`
}

func (o computersOverlay) apply(dst *ComputersConfig) {
	if o.LocalRunAgents != nil {
		dst.LocalRunAgents = *o.LocalRunAgents
	}
	if o.LoadBalancing != nil {
		dst.LoadBalancing = *o.LoadBalancing
	}
	if o.LocalLoadBalancingWeight != nil {
		dst.LocalLoadBalancingWeight = *o.LocalLoadBalancingWeight
	}
}
