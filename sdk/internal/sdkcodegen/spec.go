package sdkcodegen

// ResourceSpec declares metadata required to generate Compozy SDK helpers.
type ResourceSpec struct {
	Name           string
	PluralName     string
	PackagePath    string
	ImportAlias    string
	SDK2Package    string
	TypeName       string
	BuilderField   string
	IsSlice        bool
	FileExtensions []string
}

// ResourceSpecs contains every Compozy resource supported by the generator.
var ResourceSpecs = []ResourceSpec{
	{
		Name:           "Project",
		PluralName:     "Projects",
		PackagePath:    "github.com/compozy/compozy/engine/project",
		ImportAlias:    "engineproject",
		SDK2Package:    "github.com/compozy/sdk/project",
		TypeName:       "Config",
		BuilderField:   "project",
		IsSlice:        false,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Workflow",
		PluralName:     "Workflows",
		PackagePath:    "github.com/compozy/compozy/engine/workflow",
		ImportAlias:    "engineworkflow",
		SDK2Package:    "github.com/compozy/sdk/workflow",
		TypeName:       "Config",
		BuilderField:   "workflows",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Agent",
		PluralName:     "Agents",
		PackagePath:    "github.com/compozy/compozy/engine/agent",
		ImportAlias:    "engineagent",
		SDK2Package:    "github.com/compozy/sdk/agent",
		TypeName:       "Config",
		BuilderField:   "agents",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Tool",
		PluralName:     "Tools",
		PackagePath:    "github.com/compozy/compozy/engine/tool",
		ImportAlias:    "enginetool",
		SDK2Package:    "github.com/compozy/sdk/tool",
		TypeName:       "Config",
		BuilderField:   "tools",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Knowledge",
		PluralName:     "KnowledgeBases",
		PackagePath:    "github.com/compozy/compozy/engine/knowledge",
		ImportAlias:    "engineknowledge",
		SDK2Package:    "github.com/compozy/sdk/knowledge",
		TypeName:       "BaseConfig",
		BuilderField:   "knowledgeBases",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Memory",
		PluralName:     "Memories",
		PackagePath:    "github.com/compozy/compozy/engine/memory",
		ImportAlias:    "enginememory",
		SDK2Package:    "github.com/compozy/sdk/memory",
		TypeName:       "Config",
		BuilderField:   "memories",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "MCP",
		PluralName:     "MCPs",
		PackagePath:    "github.com/compozy/compozy/engine/mcp",
		ImportAlias:    "enginemcp",
		SDK2Package:    "github.com/compozy/sdk/mcp",
		TypeName:       "Config",
		BuilderField:   "mcps",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Schema",
		PluralName:     "Schemas",
		PackagePath:    "github.com/compozy/compozy/engine/schema",
		ImportAlias:    "engineschema",
		SDK2Package:    "github.com/compozy/sdk/schema",
		TypeName:       "Schema",
		BuilderField:   "schemas",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Model",
		PluralName:     "Models",
		PackagePath:    "github.com/compozy/compozy/engine/core",
		ImportAlias:    "enginecore",
		SDK2Package:    "github.com/compozy/sdk/model",
		TypeName:       "ProviderConfig",
		BuilderField:   "models",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Schedule",
		PluralName:     "Schedules",
		PackagePath:    "github.com/compozy/compozy/engine/project/schedule",
		ImportAlias:    "projectschedule",
		SDK2Package:    "github.com/compozy/sdk/schedule",
		TypeName:       "Config",
		BuilderField:   "schedules",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
	{
		Name:           "Webhook",
		PluralName:     "Webhooks",
		PackagePath:    "github.com/compozy/compozy/engine/webhook",
		ImportAlias:    "enginewebhook",
		SDK2Package:    "github.com/compozy/sdk/webhook",
		TypeName:       "Config",
		BuilderField:   "webhooks",
		IsSlice:        true,
		FileExtensions: []string{".yaml", ".yml"},
	},
}
