package contract

// ExtensionInstallPreviewPayload is the mutation-free install confirmation contract.
type ExtensionInstallPreviewPayload struct {
	DigestSHA256             string                                   `json:"digest_sha256,omitempty"`
	Inputs                   []MarketplaceInputPayload                `json:"inputs"`
	Name                     string                                   `json:"name"`
	DeclaredProfiles         []ExtensionInstallDeclaredProfilePayload `json:"declared_profiles"`
	Placements               []ExtensionPlacementPayload              `json:"placements"`
	NetworkRequirementDigest string                                   `json:"network_requirement_digest,omitempty"`
}

// ExtensionInstallDeclaredProfilePayload describes one profile bind or creation before install.
type ExtensionInstallDeclaredProfilePayload struct {
	Name        string                         `json:"name"`
	Create      bool                           `json:"create"`
	Credentials []ProfileCredentialRequirement `json:"credentials,omitempty"`
}
