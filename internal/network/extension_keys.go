package network

const (
	// ExtensionKeyCapabilitiesBrief carries the compact peer capability projection.
	ExtensionKeyCapabilitiesBrief = "compozy.capabilities_brief"
	// ExtensionKeyInclude requests optional envelope projections.
	ExtensionKeyInclude = "compozy.include"
	// ExtensionKeyCapabilityIDs narrows capability discovery to selected IDs.
	ExtensionKeyCapabilityIDs = "compozy.capability_ids"
	// ExtensionKeyCapabilityCatalog carries the rich capability projection.
	ExtensionKeyCapabilityCatalog = "compozy.capability_catalog"
	// ExtensionKeyWorkflowID correlates an envelope with its workflow.
	ExtensionKeyWorkflowID = "compozy.workflow_id"
	// ExtensionKeyHandoffVersion records the handoff contract version.
	ExtensionKeyHandoffVersion = "compozy.handoff_version"
	// ExtensionKeyHandoffDigest records the handoff content digest.
	ExtensionKeyHandoffDigest = "compozy.handoff_digest"
	// ExtensionKeyHandoffSource records the handoff source identity.
	ExtensionKeyHandoffSource = "compozy.handoff_source"
)
