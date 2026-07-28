package spec

func memoryDecisionQueryParams() []ParameterSpec {
	return []ParameterSpec{
		queryParam("op", "Controller decision op", false),
		queryParam("filename", "Target memory filename", false),
		dateTimeQueryParam("since", "Only decisions since this timestamp"),
		intQueryParam("limit", "Maximum number of decisions to return"),
	}
}
