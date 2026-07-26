package spec

func logFilterQueryParams() []ParameterSpec {
	return []ParameterSpec{
		queryParam("workspace_id", "Workspace id", false),
		queryParam("session_id", "Session id", false),
		queryParam("agent_name", "Agent name", false),
		queryParam("type", "Event type", false),
		queryParam("run", "Task run id", false),
		queryParam("actor", "Actor as kind:id", false),
		queryParam("actor_kind", "Actor kind", false),
		queryParam("actor_id", "Actor id", false),
		queryParam("provider", "Provider id projected at event write time", false),
		queryParam("outcome", "Event registry outcome", false),
		queryParam("component", "Event registry component", false),
		boolQueryParam("error_only", "Return warning and failure outcomes only"),
		intQueryParam("after_seq", "Return rows after this event summary sequence"),
		dateTimeQueryParam("since", "Only logs emitted since this timestamp"),
		intQueryParam("limit", "Maximum number of records to return"),
	}
}

func logStreamQueryParams() []ParameterSpec {
	params := logFilterQueryParams()
	return append(params, boolQueryParam("replay", "Replay bounded retained logs before live polling"))
}
