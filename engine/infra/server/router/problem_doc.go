package router

// ProblemDocument models an RFC 7807 error envelope for API responses.
type ProblemDocument struct {
	Type     string `json:"type,omitempty"     example:"about:blank"`
	Title    string `json:"title"              example:"Bad Request"`
	Status   int    `json:"status"             example:"400"`
	Detail   string `json:"detail,omitempty"   example:"Invalid cursor parameter"`
	Instance string `json:"instance,omitempty" example:"/api/v0/workflows"`
	Code     string `json:"code,omitempty"     example:"invalid_cursor"`
}
