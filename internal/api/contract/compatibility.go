package contract

type CompatibilityNote struct {
	Surface            string
	StableJSONFields   []string
	AdapterRequiredFor []string
	Notes              []string
}

var RunCompatibilityNotes = []CompatibilityNote{
	{
		Surface: "RunSnapshotResponse",
		StableJSONFields: []string{
			"run",
			"jobs",
			"transcript",
			"usage",
			"shutdown",
			"incomplete",
			"next_cursor",
		},
		AdapterRequiredFor: []string{
			"renaming or re-nesting top-level snapshot fields",
			"changing cursor encoding away from RFC3339Nano|zero-padded-sequence",
		},
		Notes: []string{
			"`next_cursor` stays top-level and string-encoded even though `RunSnapshot.NextCursor` is not JSON-backed",
			"additive fields are allowed, but existing fields must remain stable for downstream readers",
		},
	},
	{
		Surface: "RunEventPageResponse",
		StableJSONFields: []string{
			"events",
			"next_cursor",
			"has_more",
		},
		AdapterRequiredFor: []string{
			"changing pagination cursor encoding",
			"wrapping event records in a new container shape",
		},
		Notes: []string{
			"`events` remains a direct array of canonical `pkg/compozy/events.Event` records",
		},
	},
	{
		Surface: "pkg/compozy/runs",
		StableJSONFields: []string{
			"run",
			"jobs",
			"next_cursor",
		},
		AdapterRequiredFor: []string{
			"removing snapshot job summaries used to infer IDE/model data",
			"changing run summary field names such as `run_id`, `status`, or `presentation_mode`",
		},
		Notes: []string{
			"public run-reader consumers currently rely on snapshot compatibility and must tolerate additive fields like `usage`, `shutdown`, and `incomplete`",
		},
	},
}
