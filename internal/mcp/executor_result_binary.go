package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/diagnostics"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const maxMCPJSONBinaryDepth = 100

func encodeMCPBinaryData(value []byte, field string) (json.RawMessage, bool, error) {
	redactedBytes := diagnostics.RedactExactBytes(value)
	encoded, encodedRedaction, err := encodeMCPResultJSON(redactedBytes, field)
	if err != nil {
		return nil, false, err
	}
	return encoded, encodedRedaction || !bytes.Equal(redactedBytes, value), nil
}

func encodeMCPResultJSON(value any, field string) (json.RawMessage, bool, error) {
	redactedValue, semanticRedaction, err := redactMCPJSONValueBytes(value, 0)
	if err != nil {
		return nil, false, fmt.Errorf("mcp: redact %s bytes: %w", field, err)
	}
	encoded, err := json.Marshal(redactedValue)
	if err != nil {
		return nil, false, fmt.Errorf("mcp: encode %s: %w", field, err)
	}
	redacted, err := diagnostics.RedactExactBinaryJSON(encoded)
	if err != nil {
		return nil, false, fmt.Errorf("mcp: redact %s: %w", field, err)
	}
	return redacted, semanticRedaction || !bytes.Equal(redacted, encoded), nil
}

func redactMCPContentBytes(content mcpsdk.Content) (mcpsdk.Content, bool, error) {
	switch typed := content.(type) {
	case *mcpsdk.TextContent:
		return redactMCPTextContentBytes(typed)
	case *mcpsdk.ImageContent:
		return redactMCPImageContentBytes(typed)
	case *mcpsdk.AudioContent:
		return redactMCPAudioContentBytes(typed)
	case *mcpsdk.ResourceLink:
		return redactMCPResourceLinkBytes(typed)
	case *mcpsdk.EmbeddedResource:
		return redactMCPEmbeddedResourceBytes(typed)
	default:
		return content, false, nil
	}
}

func redactMCPTextContentBytes(content *mcpsdk.TextContent) (mcpsdk.Content, bool, error) {
	if content == nil {
		return nil, false, errors.New("mcp: text content is nil")
	}
	redacted := *content
	meta, changed, err := redactMCPMetaBytes(content.Meta)
	if err != nil {
		return nil, false, err
	}
	redacted.Meta = meta
	return &redacted, changed, nil
}

func redactMCPImageContentBytes(content *mcpsdk.ImageContent) (mcpsdk.Content, bool, error) {
	if content == nil {
		return nil, false, errors.New("mcp: image content is nil")
	}
	redacted := *content
	redacted.Data = diagnostics.RedactExactBytes(content.Data)
	meta, metaChanged, err := redactMCPMetaBytes(content.Meta)
	if err != nil {
		return nil, false, err
	}
	redacted.Meta = meta
	return &redacted, metaChanged || !bytes.Equal(redacted.Data, content.Data), nil
}

func redactMCPAudioContentBytes(content *mcpsdk.AudioContent) (mcpsdk.Content, bool, error) {
	if content == nil {
		return nil, false, errors.New("mcp: audio content is nil")
	}
	redacted := *content
	redacted.Data = diagnostics.RedactExactBytes(content.Data)
	meta, metaChanged, err := redactMCPMetaBytes(content.Meta)
	if err != nil {
		return nil, false, err
	}
	redacted.Meta = meta
	return &redacted, metaChanged || !bytes.Equal(redacted.Data, content.Data), nil
}

func redactMCPResourceLinkBytes(content *mcpsdk.ResourceLink) (mcpsdk.Content, bool, error) {
	if content == nil {
		return nil, false, errors.New("mcp: resource link content is nil")
	}
	redacted := *content
	meta, changed, err := redactMCPMetaBytes(content.Meta)
	if err != nil {
		return nil, false, err
	}
	redacted.Meta = meta
	return &redacted, changed, nil
}

func redactMCPEmbeddedResourceBytes(content *mcpsdk.EmbeddedResource) (mcpsdk.Content, bool, error) {
	if content == nil {
		return nil, false, errors.New("mcp: embedded resource content is nil")
	}
	redacted := *content
	meta, metaChanged, err := redactMCPMetaBytes(content.Meta)
	if err != nil {
		return nil, false, err
	}
	redacted.Meta = meta
	if content.Resource == nil {
		return &redacted, metaChanged, nil
	}
	resource := *content.Resource
	resource.Blob = diagnostics.RedactExactBytes(content.Resource.Blob)
	resourceMeta, resourceMetaChanged, err := redactMCPMetaBytes(content.Resource.Meta)
	if err != nil {
		return nil, false, err
	}
	resource.Meta = resourceMeta
	redacted.Resource = &resource
	return &redacted, metaChanged || resourceMetaChanged ||
		!bytes.Equal(resource.Blob, content.Resource.Blob), nil
}

func redactMCPMetaBytes(meta mcpsdk.Meta) (mcpsdk.Meta, bool, error) {
	return redactMCPMetaValueBytes(meta, 0)
}

func redactMCPMetaValueBytes(meta mcpsdk.Meta, depth int) (mcpsdk.Meta, bool, error) {
	if meta == nil {
		return nil, false, nil
	}
	redacted := make(mcpsdk.Meta, len(meta))
	changed := false
	for key, item := range meta {
		redactedItem, itemChanged, err := redactMCPJSONValueBytes(item, depth+1)
		if err != nil {
			return nil, false, err
		}
		redacted[key] = redactedItem
		changed = changed || itemChanged
	}
	return redacted, changed, nil
}

func redactMCPJSONValueBytes(value any, depth int) (any, bool, error) {
	if depth > maxMCPJSONBinaryDepth {
		return nil, false, errors.New("binary JSON value exceeds maximum nesting depth")
	}
	switch typed := value.(type) {
	case nil:
		return nil, false, nil
	case json.RawMessage:
		redacted, err := diagnostics.RedactExactBinaryJSON(typed)
		return redacted, !bytes.Equal(redacted, typed), err
	case []byte:
		redacted := diagnostics.RedactExactBytes(typed)
		return redacted, !bytes.Equal(redacted, typed), nil
	case mcpsdk.Meta:
		return redactMCPMetaValueBytes(typed, depth)
	case map[string]any:
		return redactMCPMapValueBytes(typed, depth)
	case []any:
		return redactMCPSliceValueBytes(typed, depth)
	default:
		return value, false, nil
	}
}

func redactMCPMapValueBytes(values map[string]any, depth int) (map[string]any, bool, error) {
	if values == nil {
		return nil, false, nil
	}
	redacted := make(map[string]any, len(values))
	changed := false
	for key, item := range values {
		redactedItem, itemChanged, err := redactMCPJSONValueBytes(item, depth+1)
		if err != nil {
			return nil, false, err
		}
		redacted[key] = redactedItem
		changed = changed || itemChanged
	}
	return redacted, changed, nil
}

func redactMCPSliceValueBytes(values []any, depth int) ([]any, bool, error) {
	if values == nil {
		return nil, false, nil
	}
	redacted := make([]any, len(values))
	changed := false
	for index := range values {
		redactedItem, itemChanged, err := redactMCPJSONValueBytes(values[index], depth+1)
		if err != nil {
			return nil, false, err
		}
		redacted[index] = redactedItem
		changed = changed || itemChanged
	}
	return redacted, changed, nil
}
