package knowledgerouter

import (
	"github.com/compozy/compozy/engine/core/httpdto"
	"github.com/compozy/compozy/engine/knowledge"
)

type KnowledgeBaseListResponse struct {
	KnowledgeBases []map[string]any    `json:"knowledge_bases"`
	Page           httpdto.PageInfoDTO `json:"page"`
}

type KnowledgeBaseResponse struct {
	KnowledgeBase map[string]any `json:"knowledge_base"`
}

type KnowledgeIngestRequest struct {
	Strategy string `json:"strategy"`
}

type KnowledgeIngestResponse struct {
	KnowledgeBaseID string `json:"knowledge_base_id"`
	BindingID       string `json:"binding_id"`
	Documents       int    `json:"documents"`
	Chunks          int    `json:"chunks"`
	Persisted       int    `json:"persisted"`
}

type KnowledgeQueryRequest struct {
	Query    string            `json:"query"`
	TopK     int               `json:"top_k,omitempty"`
	MinScore *float64          `json:"min_score,omitempty"`
	Filters  map[string]string `json:"filters,omitempty"`
}

type KnowledgeQueryResponse struct {
	Matches []KnowledgeMatch `json:"matches"`
}

type KnowledgeMatch struct {
	BindingID     string         `json:"binding_id"`
	Content       string         `json:"content"`
	Score         float64        `json:"score"`
	TokenEstimate int            `json:"token_estimate"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

func toKnowledgeMatch(ctx knowledge.RetrievedContext) KnowledgeMatch {
	return KnowledgeMatch{
		BindingID:     ctx.BindingID,
		Content:       ctx.Content,
		Score:         ctx.Score,
		TokenEstimate: ctx.TokenEstimate,
		Metadata:      ctx.Metadata,
	}
}
