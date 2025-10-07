package core

type KnowledgeBinding struct {
	ID        string            `json:"id"                   yaml:"id"                   mapstructure:"id"`
	TopK      *int              `json:"top_k,omitempty"      yaml:"top_k,omitempty"      mapstructure:"top_k,omitempty"`
	MinScore  *float64          `json:"min_score,omitempty"  yaml:"min_score,omitempty"  mapstructure:"min_score,omitempty"`
	MaxTokens *int              `json:"max_tokens,omitempty" yaml:"max_tokens,omitempty" mapstructure:"max_tokens,omitempty"`
	InjectAs  string            `json:"inject_as,omitempty"  yaml:"inject_as,omitempty"  mapstructure:"inject_as,omitempty"`
	Fallback  string            `json:"fallback,omitempty"   yaml:"fallback,omitempty"   mapstructure:"fallback,omitempty"`
	Filters   map[string]string `json:"filters,omitempty"    yaml:"filters,omitempty"    mapstructure:"filters,omitempty"`
}

func (b *KnowledgeBinding) Clone() KnowledgeBinding {
	if b == nil {
		return KnowledgeBinding{}
	}
	c := KnowledgeBinding{
		ID:       b.ID,
		InjectAs: b.InjectAs,
		Fallback: b.Fallback,
	}
	if b.TopK != nil {
		topk := *b.TopK
		c.TopK = &topk
	}
	if b.MinScore != nil {
		minScore := *b.MinScore
		c.MinScore = &minScore
	}
	if b.MaxTokens != nil {
		maxTokens := *b.MaxTokens
		c.MaxTokens = &maxTokens
	}
	if len(b.Filters) > 0 {
		c.Filters = make(map[string]string, len(b.Filters))
		for k, v := range b.Filters {
			c.Filters[k] = v
		}
	}
	return c
}
