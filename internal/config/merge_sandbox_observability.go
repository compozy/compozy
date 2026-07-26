package config

func (o sandboxOverlay) Apply(dst *SandboxProfile) {
	if o.Backend != nil {
		dst.Backend = *o.Backend
	}
	if o.SyncMode != nil {
		dst.SyncMode = *o.SyncMode
	}
	if o.Persistence != nil {
		dst.Persistence = *o.Persistence
	}
	if o.RuntimeRoot != nil {
		dst.RuntimeRoot = *o.RuntimeRoot
	}
	if o.Env != nil {
		dst.Env = mergeStringMaps(dst.Env, *o.Env)
	}
	if o.SecretEnv != nil {
		dst.SecretEnv = mergeStringMaps(dst.SecretEnv, *o.SecretEnv)
	}
	o.Network.Apply(&dst.Network)
	o.Daytona.Apply(&dst.Daytona)
}

func (o networkProfileOverlay) Apply(dst *NetworkProfile) {
	if o.AllowPublicIngress != nil {
		dst.AllowPublicIngress = *o.AllowPublicIngress
	}
	if o.AllowOutbound != nil {
		dst.AllowOutbound = *o.AllowOutbound
	}
	if o.AllowList != nil {
		dst.AllowList = append([]string(nil), (*o.AllowList)...)
	}
	if o.DenyList != nil {
		dst.DenyList = append([]string(nil), (*o.DenyList)...)
	}
	if o.Required != nil {
		dst.Required = *o.Required
	}
}

func (o daytonaProfileOverlay) Apply(dst *DaytonaProfile) {
	if o.APIURL != nil {
		dst.APIURL = *o.APIURL
	}
	if o.Target != nil {
		dst.Target = *o.Target
	}
	if o.Image != nil {
		dst.Image = *o.Image
	}
	if o.Snapshot != nil {
		dst.Snapshot = *o.Snapshot
	}
	if o.Class != nil {
		dst.Class = *o.Class
	}
	if o.AutoStop != nil {
		dst.AutoStop = *o.AutoStop
	}
	if o.AutoArchive != nil {
		dst.AutoArchive = *o.AutoArchive
	}
}

func (o observabilityOverlay) Apply(dst *ObservabilityConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.RetentionDays != nil {
		dst.RetentionDays = *o.RetentionDays
	}
	if o.MaxGlobalBytes != nil {
		dst.MaxGlobalBytes = *o.MaxGlobalBytes
	}
	if o.AgentProbeTimeout != nil {
		dst.AgentProbeTimeout = *o.AgentProbeTimeout
	}
	o.Transcripts.Apply(&dst.Transcripts)
}

func (o observabilityTranscriptsOverlay) Apply(dst *ObservabilityTranscriptConfig) {
	if o.Enabled != nil {
		dst.Enabled = *o.Enabled
	}
	if o.SegmentBytes != nil {
		dst.SegmentBytes = *o.SegmentBytes
	}
	if o.MaxBytesPerSession != nil {
		dst.MaxBytesPerSession = *o.MaxBytesPerSession
	}
}
