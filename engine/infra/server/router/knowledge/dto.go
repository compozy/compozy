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
	Strategy string `json:"strategy" example:"replace"`
}

type KnowledgeIngestResponse struct {
	KnowledgeBaseID string `json:"knowledge_base_id" example:"support"`
	BindingID       string `json:"binding_id"        example:"binding-123"`
	Documents       int    `json:"documents"         example:"2"`
	Chunks          int    `json:"chunks"            example:"16"`
	Persisted       int    `json:"persisted"         example:"16"`
}

// KnowledgeQueryRequest captures the payload required to query a knowledge base.
type KnowledgeQueryRequest struct {
	Query    string            `json:"query"               binding:"required" example:"How do I reset my password?"`
	TopK     int               `json:"top_k,omitempty"                        example:"5"`
	MinScore *float64          `json:"min_score,omitempty"                    example:"0.4"`
	Filters  map[string]string `json:"filters,omitempty"`
}

type KnowledgeQueryResponse struct {
	Matches []KnowledgeMatch `json:"matches"`
}

type KnowledgeMatch struct {
	BindingID     string         `json:"binding_id"         example:"binding-123"`
	Content       string         `json:"content"            example:"Reset your password by visiting the account settings page."`
	Score         float64        `json:"score"              example:"0.83"`
	TokenEstimate int            `json:"token_estimate"     example:"120"`
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
