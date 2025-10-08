package llmadapter

import "context"

type clientCtxKey string

const contextKeyLLMClient clientCtxKey = "llm.adapter.client"

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
