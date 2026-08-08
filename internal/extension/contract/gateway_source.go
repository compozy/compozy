package contract

import "time"

// ConnectivityEstablishRequest asks a provider to forward one tier to the daemon listener.
type ConnectivityEstablishRequest struct {
	Tier          string    `json:"tier"`
	ForwardTarget string    `json:"forward_target"`
	ChallengePath string    `json:"challenge_path"`
	Deadline      time.Time `json:"deadline"`
}

// ConnectivityStatusRequest selects one tier whose reachability must be reported.
type ConnectivityStatusRequest struct {
	Tier string `json:"tier"`
}

// ConnectivityTeardownRequest asks a provider to stop forwarding one tier.
type ConnectivityTeardownRequest struct {
	Tier     string    `json:"tier"`
	Deadline time.Time `json:"deadline"`
}

// ConnectivityReachability is the untrusted provider response verified by the daemon.
type ConnectivityReachability struct {
	Tier      string                           `json:"tier"`
	Endpoints []ConnectivityAdvertisedEndpoint `json:"endpoints"`
	Health    string                           `json:"health"`
	Reason    string                           `json:"reason,omitempty"`
}

// ConnectivityAdvertisedEndpoint is one provider-owned external address.
type ConnectivityAdvertisedEndpoint struct {
	URL          string `json:"url"`
	Scheme       string `json:"scheme"`
	SchemePolicy string `json:"scheme_policy,omitempty"`
	Stability    string `json:"stability"`
}

// ConnectivityTeardownResponse acknowledges that the tier stopped forwarding.
type ConnectivityTeardownResponse struct {
	Stopped bool `json:"stopped"`
}
