package providerusage

import (
	"net/http"
	"time"
)

type RegistryOptions struct {
	BaseURLs  map[string]string
	Transport http.RoundTripper
	Now       func() time.Time
	Keychain  KeychainReader
}

// DefaultRegistry constructs the supported collectors without probing credentials or providers.
func DefaultRegistry(options RegistryOptions) Registry {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	collectorOptions := func(id string) []CollectorOption {
		result := []CollectorOption{WithCollectorTransport(options.Transport), WithCollectorClock(now)}
		if base := options.BaseURLs[id]; base != "" {
			result = append(result, WithCollectorBaseURL(base))
		}
		return result
	}
	claudeOptions := []ClaudeOption{WithClaudeClock(now)}
	base := options.BaseURLs["claude"]
	if base == "" {
		base = "https://api.anthropic.com"
	}
	claudeOptions = append(claudeOptions, WithClaudeHTTP(base, options.Transport))
	if options.Keychain != nil {
		claudeOptions = append(claudeOptions, WithClaudeKeychain(options.Keychain))
	}
	collectors := []Collector{
		NewCodexCollector(
			CodexOptions{BaseURL: options.BaseURLs[codexProviderID], Transport: options.Transport, Now: now},
		),
		NewClaudeCollector(claudeOptions...),
		NewOpenRouterCollector(collectorOptions("openrouter")...),
		NewVercelCollector(collectorOptions("vercel-ai-gateway")...),
		NewZaiCollector(collectorOptions("zai")...),
		NewKimiCollector(collectorOptions("moonshot")...),
		NewKimiCLICollector(collectorOptions("kimi-cli")...),
		NewCursorCollector(collectorOptions("cursor")...),
		NewGeminiCollector(collectorOptions("gemini")...),
	}
	registry := make(Registry, len(collectors))
	for _, collector := range collectors {
		registry[collector.ProviderID()] = collector
	}
	return registry
}
