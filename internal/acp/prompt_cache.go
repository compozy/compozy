package acp

import (
	"net/url"
	"strings"

	acpsdk "github.com/coder/acp-go-sdk"
)

const (
	promptCacheControlKey       = "cache_control"
	promptCacheTypeKey          = "type"
	promptCacheTTLKey           = "ttl"
	promptCacheTypeEphemeral    = "ephemeral"
	promptCacheLongTTL          = "1h"
	promptCacheAnthropicRuntime = "anthropic"
)

type promptCacheControl struct {
	Type string
	TTL  string
}

func promptCacheControlForStartOpts(opts StartOpts) *promptCacheControl {
	providerName := strings.ToLower(strings.TrimSpace(opts.ProviderName))
	runtimeProvider := providerName
	baseURL := ""
	if opts.ProviderConfig != nil {
		runtimeProvider = strings.ToLower(strings.TrimSpace(opts.ProviderConfig.RuntimeProviderName(providerName)))
		baseURL = strings.TrimSpace(opts.ProviderConfig.BaseURL)
	}
	if providerName != "claude" && runtimeProvider != promptCacheAnthropicRuntime {
		return nil
	}

	control := promptCacheControl{Type: promptCacheTypeEphemeral}
	if promptCacheLongTTLEndpoint(baseURL) {
		control.TTL = promptCacheLongTTL
	}
	return &control
}

func textBlockWithPromptCacheControl(text string, control *promptCacheControl) acpsdk.ContentBlock {
	block := acpsdk.TextBlock(text)
	if control == nil || block.Text == nil {
		return block
	}
	cacheControl := map[string]any{
		promptCacheTypeKey: control.Type,
	}
	if strings.TrimSpace(control.TTL) != "" {
		cacheControl[promptCacheTTLKey] = strings.TrimSpace(control.TTL)
	}
	block.Text.Meta = map[string]any{
		promptCacheControlKey: cacheControl,
	}
	return block
}

func promptCacheLongTTLEndpoint(rawURL string) bool {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return false
	}
	host := strings.ToLower(strings.TrimSpace(parsed.Hostname()))
	switch {
	case host == "api.anthropic.com":
		return true
	case host == "aiplatform.googleapis.com":
		return true
	case strings.HasSuffix(host, "-aiplatform.googleapis.com"):
		return true
	default:
		return false
	}
}
