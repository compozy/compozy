package agent

import (
	"context"
	"errors"
	"fmt"

	"dario.cat/mergo"

	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/memory" // Added for memory.MemoryReference
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/ref"
	"github.com/mitchellh/mapstructure" // For parsing Level 3 memories
)

type Config struct {
	Resource     string              `json:"resource,omitempty"       yaml:"resource,omitempty"       mapstructure:"resource,omitempty"`
	ID           string              `json:"id"                       yaml:"id"                       mapstructure:"id"                       validate:"required"`
	Config       core.ProviderConfig `json:"config"                   yaml:"config"                   mapstructure:"config"                   validate:"required"`
	Instructions string              `json:"instructions"             yaml:"instructions"             mapstructure:"instructions"             validate:"required"`
	Actions      []*ActionConfig     `json:"actions,omitempty"        yaml:"actions,omitempty"        mapstructure:"actions,omitempty"`
	With         *core.Input         `json:"with,omitempty"           yaml:"with,omitempty"           mapstructure:"with,omitempty"`
	Env          *core.EnvMap        `json:"env,omitempty"            yaml:"env,omitempty"            mapstructure:"env,omitempty"`
	// When defined here, the agent will have toolChoice defined as "auto"
	Tools         []tool.Config `json:"tools,omitempty"          yaml:"tools,omitempty"          mapstructure:"tools,omitempty"`
	MCPs          []mcp.Config  `json:"mcps,omitempty"           yaml:"mcps,omitempty"           mapstructure:"mcps,omitempty"`
	MaxIterations int           `json:"max_iterations,omitempty" yaml:"max_iterations,omitempty" mapstructure:"max_iterations,omitempty"`
	JSONMode      bool          `json:"json_mode"                yaml:"json_mode"                mapstructure:"json_mode"`

	// Memory configuration fields
	// Level 1: memory: "customer-support-context", memory_key: "key-template"
	// Level 2: memory: true, memories: ["id1", "id2"], memory_key: "shared-key-template"
	// Level 3: memories: [{id: "id1", mode: "read-write", key: "template1"}, {id: "id2", mode: "read-only", key: "template2"}]
	Memory    interface{} `json:"memory,omitempty"     yaml:"memory,omitempty"     mapstructure:"memory,omitempty"`    // string (L1) or bool (L2)
	Memories  interface{} `json:"memories,omitempty"   yaml:"memories,omitempty"   mapstructure:"memories,omitempty"`  // []string (L2) or []map[string]interface{} (L3) / []memory.MemoryReference (L3 after parsing)
	MemoryKey string      `json:"memory_key,omitempty" yaml:"memory_key,omitempty" mapstructure:"memory_key,omitempty"` // string (L1, L2)

	// Internal field to store normalized memory references after parsing
	resolvedMemoryReferences []memory.MemoryReference `json:"-" yaml:"-" mapstructure:"-"`

	filePath string
	CWD      *core.PathCWD
}

// GetResolvedMemoryReferences returns the normalized memory configurations.
// This should be called after Validate() has run.
func (a *Config) GetResolvedMemoryReferences() []memory.MemoryReference {
	return a.resolvedMemoryReferences
}

func (a *Config) Component() core.ConfigType {
	return core.ConfigAgent
}

func (a *Config) GetFilePath() string {
	return a.filePath
}

func (a *Config) SetFilePath(path string) {
	a.filePath = path
}

func (a *Config) SetCWD(path string) error {
	CWD, err := core.CWDFromPath(path)
	if err != nil {
		return err
	}
	a.CWD = CWD
	for i := range a.Actions {
		if err := a.Actions[i].SetCWD(path); err != nil {
			return err
		}
	}
	return nil
}

func (a *Config) GetCWD() *core.PathCWD {
	return a.CWD
}

func (a *Config) GetInput() *core.Input {
	if a.With == nil {
		a.With = &core.Input{}
	}
	return a.With
}

func (a *Config) GetEnv() core.EnvMap {
	if a.Env == nil {
		a.Env = &core.EnvMap{}
		return *a.Env
	}
	return *a.Env
}

func (a *Config) HasSchema() bool {
	return false
}

func (a *Config) GetMaxIterations() int {
	if a.MaxIterations == 0 {
		return 5
	}
	return a.MaxIterations
}

func (a *Config) normalizeAndValidateMemoryConfig() error {
	// Default mode for simplified configurations
	const defaultMemoryMode = "read-write"

	// Level 3: memories: [{id: "id1", mode: "append", key: "custom"}, ...]
	if memoriesList, ok := a.Memories.([]interface{}); ok && len(memoriesList) > 0 {
		// Check if the first element suggests it's a list of maps (Level 3)
		if _, firstIsMap := memoriesList[0].(map[string]interface{}); firstIsMap {
			if a.Memory != nil {
				return fmt.Errorf("cannot use 'memory' field (Level 1 or 2) when 'memories' is a list of objects (Level 3)")
			}
			refs := make([]memory.MemoryReference, 0, len(memoriesList))
			for i, memInterface := range memoriesList {
				var ref memory.MemoryReference
				// Use mapstructure to decode into memory.MemoryReference
				decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
					Result:           &ref,
					WeaklyTypedInput: true, // Allow some flexibility in types from YAML
				})
				if err != nil {
					return fmt.Errorf("failed to create mapstructure decoder for memory reference %d: %w", i, err)
				}
				if err := decoder.Decode(memInterface); err != nil {
					return fmt.Errorf("failed to parse memory reference %d: %w. Ensure it has 'id' and 'key'", i, err)
				}

				if ref.ID == "" {
					return fmt.Errorf("memory reference %d missing required 'id' field", i)
				}
				if ref.Key == "" {
					return fmt.Errorf("memory reference %d (id: %s) missing required 'key' field", i, ref.ID)
				}
				if ref.Mode == "" {
					ref.Mode = defaultMemoryMode
				}
				if ref.Mode != "read-write" && ref.Mode != "read-only" {
					return fmt.Errorf("memory reference %d (id: %s) has invalid mode '%s', must be 'read-write' or 'read-only'", i, ref.ID, ref.Mode)
				}
				refs = append(refs, ref)
			}
			a.resolvedMemoryReferences = refs
			return nil
		}
	}

	// Level 2: memory: true, memories: ["id1", "id2"], memory_key: "shared-key-template"
	if memoryFlag, ok := a.Memory.(bool); ok && memoryFlag {
		if a.Memories == nil {
			return fmt.Errorf("'memory: true' (Level 2) requires 'memories' field to be a non-empty list of memory IDs")
		}
		memoriesStrList, ok := a.Memories.([]interface{}) // YAML unmarshals string arrays as []interface{}
		if !ok || len(memoriesStrList) == 0 {
			// Try if it was parsed directly as []string (less common from generic YAML but possible)
			if strList, isStrList := a.Memories.([]string); isStrList && len(strList) > 0 {
				// Convert []string to []interface{} for consistent processing
				memoriesStrList = make([]interface{}, len(strList))
				for i, s := range strList {
					memoriesStrList[i] = s
				}
			} else {
				return fmt.Errorf("'memory: true' (Level 2) requires 'memories' to be a non-empty list of memory ID strings")
			}
		}

		if a.MemoryKey == "" {
			return fmt.Errorf("'memory_key' is required for Level 2 memory configuration ('memory: true')")
		}
		refs := make([]memory.MemoryReference, 0, len(memoriesStrList))
		for i, memIDInterface := range memoriesStrList {
			memID, ok := memIDInterface.(string)
			if !ok || memID == "" {
				return fmt.Errorf("memory ID at index %d in 'memories' list must be a non-empty string for Level 2 configuration", i)
			}
			refs = append(refs, memory.MemoryReference{
				ID:   memID,
				Mode: defaultMemoryMode,
				Key:  a.MemoryKey,
			})
		}
		a.resolvedMemoryReferences = refs
		return nil
	}

	// Level 1: memory: "customer-support-context", memory_key: "key-template"
	if memIDStr, ok := a.Memory.(string); ok && memIDStr != "" {
		if a.Memories != nil {
			return fmt.Errorf("cannot use 'memories' field when 'memory' is a string ID (Level 1)")
		}
		if a.MemoryKey == "" {
			return fmt.Errorf("'memory_key' is required for Level 1 memory configuration ('memory: <id>')")
		}
		a.resolvedMemoryReferences = []memory.MemoryReference{
			{
				ID:   memIDStr,
				Mode: defaultMemoryMode,
				Key:  a.MemoryKey,
			},
		}
		return nil
	}

	// No memory configuration, or 'memory: false'
	if _, isBool := a.Memory.(bool); isBool && !a.Memory.(bool) {
		// memory: false, explicit no memory
		a.resolvedMemoryReferences = []memory.MemoryReference{}
		return nil
	}

	// If a.Memory is not nil, not bool, not string, it's an invalid type for 'memory' field
	if a.Memory != nil {
		return fmt.Errorf("invalid type for 'memory' field: must be a string (ID), or boolean (true/false)")
	}
	// If a.Memories is not nil but not caught by L2/L3, it's an invalid structure for 'memories'
	if a.Memories != nil {
		return fmt.Errorf("invalid structure for 'memories' field; ensure it's a list of strings (with 'memory:true') or a list of memory reference objects")
	}

	// Default: no memory configuration
	a.resolvedMemoryReferences = []memory.MemoryReference{}
	return nil
}

func (a *Config) Validate() error {
	// Initial struct validation (for required fields like ID, Config, Instructions)
	baseValidator := schema.NewStructValidator(a)
	if err := baseValidator.Validate(); err != nil {
		return err
	}

	// Normalize and validate memory configuration first
	if err := a.normalizeAndValidateMemoryConfig(); err != nil {
		return fmt.Errorf("invalid memory configuration: %w", err)
	}

	// Now build composite validator including memory (if any)
	v := schema.NewCompositeValidator(
		schema.NewCWDValidator(a.CWD, a.ID),
		NewActionsValidator(a.Actions),
		NewAgentMemoryValidator(a.resolvedMemoryReferences /*, nil // Pass registry here in Task 4 */),
	)
	if err := v.Validate(); err != nil {
		return fmt.Errorf("agent config validation failed: %w", err)
	}

	var mcpErrors []error
	for i := range a.MCPs {
		if err := a.MCPs[i].Validate(); err != nil {
			mcpErrors = append(mcpErrors, fmt.Errorf("mcp validation error: %w", err))
		}
	}
	if len(mcpErrors) > 0 {
		return errors.Join(mcpErrors...)
	}
	return nil
}

func (a *Config) ValidateInput(_ context.Context, _ *core.Input) error {
	// Does not make sense the agent having a schema
	return nil
}

func (a *Config) ValidateOutput(_ context.Context, _ *core.Output) error {
	// Does not make sense the agent having a schema
	return nil
}

func (a *Config) Merge(other any) error {
	otherConfig, ok := other.(*Config)
	if !ok {
		return fmt.Errorf("failed to merge agent configs: %s", "invalid type for merge")
	}
	return mergo.Merge(a, otherConfig, mergo.WithOverride)
}

func (a *Config) Clone() (*Config, error) {
	if a == nil {
		return nil, nil
	}
	return core.DeepCopy(a)
}

func (a *Config) AsMap() (map[string]any, error) {
	return core.AsMapDefault(a)
}

func (a *Config) FromMap(data any) error {
	config, err := core.FromMapDefault[*Config](data)
	if err != nil {
		return err
	}
	return a.Merge(config)
}

func Load(cwd *core.PathCWD, path string) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfig[*Config](filePath)
	if err != nil {
		return nil, err
	}
	return config, nil
}

func LoadAndEval(cwd *core.PathCWD, path string, ev *ref.Evaluator) (*Config, error) {
	filePath, err := core.ResolvePath(cwd, path)
	if err != nil {
		return nil, err
	}
	config, _, err := core.LoadConfigWithEvaluator[*Config](filePath, ev)
	if err != nil {
		return nil, err
	}
	return config, nil
}
