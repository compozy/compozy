package config

import "time"

type worktreesOverlay struct {
	Root               *string        `toml:"root"`
	RunBranchNamespace *string        `toml:"run_branch_namespace"`
	CopyList           *[]string      `toml:"copy_list"`
	SetupCommand       *string        `toml:"setup_command"`
	TeardownCommand    *string        `toml:"teardown_command"`
	SetupTimeout       *time.Duration `toml:"setup_timeout"`
	TeardownTimeout    *time.Duration `toml:"teardown_timeout"`
	CheckoutTimeout    *time.Duration `toml:"checkout_timeout"`
	DiscoveryCacheTTL  *time.Duration `toml:"discovery_cache_ttl"`
}

func (o worktreesOverlay) Apply(dst *WorktreesConfig) {
	if o.Root != nil {
		dst.Root = *o.Root
	}
	if o.RunBranchNamespace != nil {
		dst.RunBranchNamespace = *o.RunBranchNamespace
	}
	if o.CopyList != nil {
		dst.CopyList = append([]string(nil), (*o.CopyList)...)
	}
	if o.SetupCommand != nil {
		dst.SetupCommand = *o.SetupCommand
	}
	if o.SetupTimeout != nil {
		dst.SetupTimeout = *o.SetupTimeout
	}
	if o.TeardownCommand != nil {
		dst.TeardownCommand = *o.TeardownCommand
	}
	if o.TeardownTimeout != nil {
		dst.TeardownTimeout = *o.TeardownTimeout
	}
	if o.CheckoutTimeout != nil {
		dst.CheckoutTimeout = *o.CheckoutTimeout
	}
	if o.DiscoveryCacheTTL != nil {
		dst.DiscoveryCacheTTL = *o.DiscoveryCacheTTL
	}
}
