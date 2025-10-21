package autoload

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/resources"
)

// configEntry represents a configuration entry in the registry
type configEntry struct {
	config any
	source string // "manual" or "autoload"
}

func registryKeyToResourceType(t string) (resources.ResourceType, bool) {
	switch strings.ToLower(strings.TrimSpace(t)) {
	case "workflow":
		return resources.ResourceWorkflow, true
	case "task":
		return resources.ResourceTask, true
	case "agent":
		return resources.ResourceAgent, true
	case "tool":
		return resources.ResourceTool, true
	case "mcp":
		return resources.ResourceMCP, true
	case "project":
		return resources.ResourceProject, true
	case "memory":
		return resources.ResourceMemory, true
	case "schema":
		return resources.ResourceSchema, true
	case "model":
		return resources.ResourceModel, true
	case "knowledge_base", "knowledge-base", "knowledgebase":
		return resources.ResourceKnowledgeBase, true
	case "embedder", "knowledge_embedder":
		return resources.ResourceEmbedder, true
	case "vector_db", "vector-db", "vectordb", "knowledge_vector_db":
		return resources.ResourceVectorDB, true
	default:
		return "", false
	}
}

// ConfigRegistry stores and manages discovered configurations
type ConfigRegistry struct {
	mu      sync.RWMutex
	configs map[string]map[string]*configEntry // type -> id -> entry
}

// NewConfigRegistry creates a new configuration registry
func NewConfigRegistry() *ConfigRegistry {
	return &ConfigRegistry{
		configs: make(map[string]map[string]*configEntry),
	}
}

// Register adds a configuration to the registry
func (r *ConfigRegistry) Register(config any, source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	resourceType, id, err := extractResourceInfo(config)
	if err != nil {
		return err
	}
	resourceType = strings.TrimSpace(strings.ToLower(resourceType))
	id = strings.TrimSpace(strings.ToLower(id))
	if resourceType == "" || id == "" {
		return core.NewError(nil, "INVALID_RESOURCE_INFO", map[string]any{
			"type": resourceType,
			"id":   id,
		})
	}
	if _, ok := r.configs[resourceType]; !ok {
		r.configs[resourceType] = make(map[string]*configEntry)
	}
	if existing, exists := r.configs[resourceType][id]; exists {
		return core.NewError(nil, "DUPLICATE_CONFIG", map[string]any{
			"type":            resourceType,
			"id":              id,
			"source":          source,
			"existing_source": existing.source,
			"suggestion":      "Check for duplicate resource IDs or use unique identifiers across configuration files",
		})
	}
	r.configs[resourceType][id] = &configEntry{
		config: config,
		source: source,
	}
	return nil
}

// Get retrieves a configuration from the registry
func (r *ConfigRegistry) Get(resourceType, id string) (any, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resourceType = strings.TrimSpace(strings.ToLower(resourceType))
	id = strings.TrimSpace(strings.ToLower(id))
	if configs, ok := r.configs[resourceType]; ok {
		if entry, ok := configs[id]; ok {
			return entry.config, nil
		}
	}
	return nil, core.NewError(nil, "RESOURCE_NOT_FOUND", map[string]any{
		"type":       resourceType,
		"id":         id,
		"suggestion": "Verify the resource exists and has been loaded by AutoLoader",
	})
}

// Count returns the total number of configurations in the registry
func (r *ConfigRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, configs := range r.configs {
		count += len(configs)
	}
	return count
}

// GetAll returns all configurations of a specific type
func (r *ConfigRegistry) GetAll(resourceType string) []any {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resourceType = strings.TrimSpace(strings.ToLower(resourceType))
	if configs, ok := r.configs[resourceType]; ok {
		result := make([]any, 0, len(configs))
		for _, entry := range configs {
			result = append(result, entry.config)
		}
		return result
	}
	return []any{} // Return empty slice instead of nil
}

// Clear removes all configurations from the registry
// Note: Clear must not be called concurrently with Register/Get/GetAll
func (r *ConfigRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.configs = make(map[string]map[string]*configEntry)
}

// extractResourceInfo extracts the resource type and ID from a configuration using reflection
func extractResourceInfo(config any) (resourceType string, id string, err error) {
	if c, ok := config.(Configurable); ok {
		return c.GetResource(), c.GetID(), nil
	}
	if configMap, ok := config.(map[string]any); ok {
		return extractResourceInfoFromMap(configMap)
	}
	v := reflect.ValueOf(config)
	if !v.IsValid() {
		return "", "", core.NewError(
			errors.New("nil or invalid configuration"),
			"NIL_CONFIG",
			nil,
		)
	}
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return "", "", core.NewError(nil, "NIL_CONFIG_POINTER", nil)
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return "", "", core.NewError(nil, "INVALID_CONFIG_TYPE", map[string]any{
			"type": fmt.Sprintf("%T", config),
		})
	}
	typeName := fmt.Sprintf("%T", config)
	resourceType = extractResourceType(v, typeName)
	if resourceType == "" {
		return "", "", core.NewError(nil, "UNKNOWN_CONFIG_TYPE", map[string]any{
			"type": typeName,
		})
	}
	id = extractID(v, typeName)
	if id == "" {
		return "", "", core.NewError(nil, "EMPTY_ID", map[string]any{
			"resource_type": resourceType,
			"config_type":   typeName,
		})
	}
	return resourceType, id, nil
}

// extractResourceType gets the resource type from config
func extractResourceType(v reflect.Value, typeName string) string {
	resourceField := v.FieldByName("Resource")
	if resourceField.IsValid() && resourceField.Kind() == reflect.String {
		resourceValue := resourceField.String()
		if resourceValue != "" {
			return resourceValue
		}
	}
	resourceTypeMap := map[string]string{
		"*workflow.Config":          string(core.ConfigWorkflow),
		"*task.Config":              string(core.ConfigTask),
		"*agent.Config":             string(core.ConfigAgent),
		"*tool.Config":              string(core.ConfigTool),
		"*mcp.Config":               string(core.ConfigMCP),
		"*project.Config":           string(core.ConfigProject),
		"*memory.Config":            string(core.ConfigMemory), // Added for memory.Config
		"task.Config":               string(core.ConfigTask),
		"task.BaseConfig":           string(core.ConfigTask),
		"memory.Config":             string(core.ConfigMemory), // Added for memory.Config by value
		"*knowledge.BaseConfig":     string(core.ConfigKnowledgeBase),
		"knowledge.BaseConfig":      string(core.ConfigKnowledgeBase),
		"*knowledge.EmbedderConfig": string(core.ConfigEmbedder),
		"knowledge.EmbedderConfig":  string(core.ConfigEmbedder),
		"*knowledge.VectorDBConfig": string(core.ConfigVectorDB),
		"knowledge.VectorDBConfig":  string(core.ConfigVectorDB),
	}
	if rt, ok := resourceTypeMap[typeName]; ok {
		return rt
	}
	return ""
}

// extractID gets the ID from config
func extractID(v reflect.Value, typeName string) string {
	idField := v.FieldByName("ID")
	if idField.IsValid() && idField.Kind() == reflect.String {
		return idField.String()
	}
	if typeName == "*project.Config" {
		nameField := v.FieldByName("Name")
		if nameField.IsValid() && nameField.Kind() == reflect.String {
			return nameField.String()
		}
	}
	baseConfigField := v.FieldByName("BaseConfig")
	if baseConfigField.IsValid() && baseConfigField.Kind() == reflect.Struct {
		idField = baseConfigField.FieldByName("ID")
		if idField.IsValid() && idField.Kind() == reflect.String {
			return idField.String()
		}
	}
	return ""
}

// extractResourceInfoFromMap extracts resource type and ID from a map configuration
func extractResourceInfoFromMap(configMap map[string]any) (resourceType string, id string, err error) {
	if resource, exists := configMap["resource"]; exists {
		if resourceStr, ok := resource.(string); ok && resourceStr != "" {
			resourceType = resourceStr
		} else {
			return "", "", core.NewError(
				errors.New("resource field must be a non-empty string"),
				"INVALID_RESOURCE_FIELD",
				map[string]any{"resource": resource},
			)
		}
	} else {
		return "", "", core.NewError(
			errors.New("configuration missing required resource field"),
			"MISSING_RESOURCE_FIELD",
			nil,
		)
	}
	if idValue, exists := configMap["id"]; exists {
		if idStr, ok := idValue.(string); ok && idStr != "" {
			id = idStr
		} else {
			return "", "", core.NewError(
				errors.New("id field must be a non-empty string"),
				"INVALID_ID_FIELD",
				map[string]any{"id": idValue},
			)
		}
	} else {
		return "", "", core.NewError(
			errors.New("configuration missing required id field"),
			"MISSING_ID_FIELD",
			nil,
		)
	}
	return resourceType, id, nil
}

// CountByType returns the number of configurations of a specific resource type
func (r *ConfigRegistry) CountByType(resourceType string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	resourceType = strings.TrimSpace(strings.ToLower(resourceType))
	configs, exists := r.configs[resourceType]
	if !exists {
		return 0
	}
	return len(configs)
}

// SyncToResourceStore publishes all registered configurations to the provided
// ResourceStore under stable (project,type,id) keys. This is intended for
// development and tests where AutoLoad discovers resources from the filesystem.
//
// Notes:
//   - We do a best-effort type mapping. When entries are map[string]any, they are
//     stored as-is; typed entries (e.g., *agent.Config) are stored as pointers.
//   - Compile/link expects typed values for agent/tool lookups; when possible,
//     prefer registering typed configs via AutoLoader. This method still stores
//     raw maps to aid schema/model indexing.
func (r *ConfigRegistry) SyncToResourceStore(ctx context.Context, project string, store resources.ResourceStore) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if store == nil {
		return fmt.Errorf("resource store is required")
	}
	if strings.TrimSpace(project) == "" {
		return fmt.Errorf("project name is required")
	}
	for t, byID := range r.configs {
		rtype, ok := registryKeyToResourceType(t)
		if !ok {
			continue
		}
		if err := r.publishTypeBucket(ctx, project, rtype, byID, store); err != nil {
			return err
		}
	}
	return nil
}

func (r *ConfigRegistry) publishTypeBucket(
	ctx context.Context,
	project string,
	rtype resources.ResourceType,
	byID map[string]*configEntry,
	store resources.ResourceStore,
) error {
	for id, entry := range byID {
		key := resources.ResourceKey{Project: project, Type: rtype, ID: id}
		if _, err := store.Put(ctx, key, entry.config); err != nil {
			return fmt.Errorf("failed to publish %s '%s' to store: %w", string(rtype), id, err)
		}
		if err := resources.WriteMetaForAutoload(ctx, store, project, rtype, id); err != nil {
			return fmt.Errorf("failed to write autoload meta for %s '%s': %w", string(rtype), id, err)
		}
	}
	return nil
}
