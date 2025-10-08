package resources

import "path/filepath"

// DirForType returns the repository subdirectory name used for the provided
// resource type. Centralized here to keep importer and exporter directory
// mappings in sync.
func DirForType(t ResourceType) (string, bool) {
	switch t {
	case ResourceWorkflow:
		return "workflows", true
	case ResourceAgent:
		return "agents", true
	case ResourceTool:
		return "tools", true
	case ResourceTask:
		return "tasks", true
	case ResourceSchema:
		return "schemas", true
	case ResourceMCP:
		return "mcps", true
	case ResourceModel:
		return "models", true
	case ResourceMemory:
		return "memories", true
	case ResourceKnowledgeBase:
		return filepath.Join("knowledge", "knowledge-bases"), true
	case ResourceEmbedder:
		return filepath.Join("knowledge", "embedders"), true
	case ResourceVectorDB:
		return filepath.Join("knowledge", "vector-dbs"), true
	case ResourceProject:
		return "project", true
	default:
		return "", false
	}
}
