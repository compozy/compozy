package cli

import "context"

type toolCatalogClient interface {
	workspaceLookupClient
	ListTools(context.Context, ToolQuery) (ToolsResponseRecord, error)
	SearchTools(context.Context, ToolSearchRequest) (ToolsResponseRecord, error)
	GetTool(context.Context, string, ToolQuery) (ToolResponseRecord, error)
	ListToolsets(context.Context, ToolQuery) (ToolsetsResponseRecord, error)
	GetToolset(context.Context, string, ToolQuery) (ToolsetResponseRecord, error)
}

type toolApprovalClient interface {
	workspaceLookupClient
	CreateToolApproval(context.Context, string, ToolApprovalRequest) (ToolApprovalRecord, error)
	SetToolApprovalGrant(
		context.Context,
		string,
		ToolApprovalGrantSetRequest,
	) (ToolApprovalGrantRecord, error)
	ListToolApprovalGrants(context.Context, string) (ToolApprovalGrantListRecord, error)
	RevokeToolApprovalGrant(context.Context, string, string) error
}

type toolInvocationClient interface {
	workspaceLookupClient
	InvokeTool(context.Context, string, ToolInvokeRequest) (ToolInvokeResponseRecord, error)
}

type toolArtifactClient interface {
	workspaceLookupClient
	ReadToolArtifact(context.Context, string, string, int64, int64) (ToolArtifactPageRecord, error)
}
