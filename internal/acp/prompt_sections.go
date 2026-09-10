package acp

import (
	"context"
	"crypto/sha256"
	"slices"
	"strings"
	"sync"
)

// PromptSection retains full context until the transport confirms its delivery.
type PromptSection struct {
	Key              string
	Content          string
	StartupContent   string
	UnchangedContent string
}

type promptSectionsKey struct{}

type promptSections struct {
	mu       sync.Mutex
	sections []PromptSection
}

// CollectPromptSections collects daemon-owned context for one dispatch attempt.
func CollectPromptSections(ctx context.Context) (context.Context, func() []PromptSection) {
	collector := &promptSections{}
	return context.WithValue(ctx, promptSectionsKey{}, collector), func() []PromptSection {
		collector.mu.Lock()
		defer collector.mu.Unlock()
		return slices.Clone(collector.sections)
	}
}

// RegisterPromptSection keeps compaction metadata out of provider-visible prompt metadata.
func RegisterPromptSection(ctx context.Context, section PromptSection) {
	collector, ok := ctx.Value(promptSectionsKey{}).(*promptSections)
	if !ok {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	collector.sections = append(collector.sections, section)
}

func (p *AgentProcess) compactPromptSections(message string, sections []PromptSection) string {
	p.systemPromptMu.Lock()
	defer p.systemPromptMu.Unlock()
	for _, section := range sections {
		if section.Key == "" || section.Content == "" || section.UnchangedContent == "" {
			continue
		}
		if strings.Count(message, section.Content) != 1 {
			continue
		}
		signature, delivered := p.deliveredSections[section.Key]
		unchanged := delivered && signature == sha256.Sum256([]byte(section.Content))
		startup := !p.systemPromptSent && section.StartupContent != "" &&
			strings.Contains(p.systemPrompt, section.StartupContent)
		if unchanged || startup {
			message = strings.Replace(message, section.Content, section.UnchangedContent, 1)
		}
	}
	return message
}

func (p *AgentProcess) markPromptSectionsDelivered(req PromptRequest) {
	p.systemPromptMu.Lock()
	defer p.systemPromptMu.Unlock()
	for _, section := range req.Sections {
		if section.Key == "" {
			continue
		}
		if section.Content == "" || strings.Count(req.Message, section.Content) != 1 {
			delete(p.deliveredSections, section.Key)
			continue
		}
		if p.deliveredSections == nil {
			p.deliveredSections = make(map[string][sha256.Size]byte)
		}
		p.deliveredSections[section.Key] = sha256.Sum256([]byte(section.Content))
	}
}

func (p *AgentProcess) forgetPromptSections(sections []PromptSection) {
	p.systemPromptMu.Lock()
	defer p.systemPromptMu.Unlock()
	for _, section := range sections {
		delete(p.deliveredSections, section.Key)
	}
}
