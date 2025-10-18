package llmadapter

import (
	"context"

	"github.com/compozy/compozy/engine/core"
)

type clientCtxKey string
type optionsCtxKey string

const (
	contextKeyLLMClient   clientCtxKey  = "llm.adapter.client"
	contextKeyCallOptions optionsCtxKey = "llm.adapter.call_options"
)

func ContextWithClient(ctx context.Context, client LLMClient) context.Context {
	if ctx == nil || client == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKeyLLMClient, client)
}

func ClientFromContext(ctx context.Context) (LLMClient, bool) {
	if ctx == nil {
		return nil, false
	}
	client, ok := ctx.Value(contextKeyLLMClient).(LLMClient)
	if !ok || client == nil {
		return nil, false
	}
	return client, true
}

func ContextWithCallOptions(ctx context.Context, opts *CallOptions) context.Context {
	if ctx == nil {
		return nil
	}
	if opts == nil {
		return ctx
	}
	copyOpts := cloneCallOptions(opts)
	return context.WithValue(ctx, contextKeyCallOptions, copyOpts)
}

func CallOptionsFromContext(ctx context.Context) (CallOptions, bool) {
	if ctx == nil {
		return CallOptions{}, false
	}
	raw, ok := ctx.Value(contextKeyCallOptions).(CallOptions)
	if !ok {
		return CallOptions{}, false
	}
	return cloneCallOptions(&raw), true
}

func cloneCallOptions(opts *CallOptions) CallOptions {
	if opts == nil {
		return CallOptions{}
	}
	out := *opts
	if len(out.StopWords) > 0 {
		out.StopWords = append([]string{}, out.StopWords...)
	}
	if len(out.Metadata) > 0 {
		out.Metadata = core.CloneMap(opts.Metadata)
	}
	return out
}
