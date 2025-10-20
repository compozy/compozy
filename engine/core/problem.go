package core

import "net/http"

// ProblemDocument models the canonical error envelope for API responses.
type ProblemDocument struct {
	Status   int    `json:"status"             example:"400"`
	Error    string `json:"error"              example:"Bad Request"`
	Details  string `json:"details,omitempty"  example:"Invalid cursor parameter"`
	Code     string `json:"code,omitempty"     example:"invalid_cursor"`
	Type     string `json:"type,omitempty"     example:"about:blank"`
	Instance string `json:"instance,omitempty" example:"/api/v0/workflows"`
}

// Problem captures the information returned in an RFC 7807 error response.
type Problem struct {
	Type     string
	Title    string
	Status   int
	Detail   string
	Instance string
	Extras   map[string]any
}

// NormalizeProblem ensures the provided problem includes canonical defaults.
func NormalizeProblem(problem *Problem) *Problem {
	if problem == nil {
		problem = &Problem{}
	}
	if problem.Status == 0 {
		problem.Status = http.StatusInternalServerError
	}
	if problem.Title == "" {
		problem.Title = http.StatusText(problem.Status)
	}
	if problem.Type == "" {
		problem.Type = "about:blank"
	}
	return problem
}

// BuildProblemBody assembles the serialized representation of the problem.
func BuildProblemBody(problem *Problem) map[string]any {
	body := map[string]any{
		"status": problem.Status,
		"error":  problem.Title,
	}
	if problem.Detail != "" {
		body["details"] = problem.Detail
	}
	if code, ok := problem.Extras["code"]; ok {
		body["code"] = code
	}
	if problem.Type != "" {
		body["type"] = problem.Type
	}
	if problem.Instance != "" {
		body["instance"] = problem.Instance
	}
	if len(problem.Extras) == 0 {
		return body
	}
	filteredExtras := make(map[string]any, len(problem.Extras))
	for key, value := range problem.Extras {
		if !isReservedProblemKey(key) {
			filteredExtras[key] = value
		}
	}
	if len(filteredExtras) == 0 {
		return body
	}
	return CopyMaps(body, filteredExtras)
}

func isReservedProblemKey(key string) bool {
	switch key {
	case "status", "error", "details", "code", "type", "instance":
		return true
	default:
		return false
	}
}
