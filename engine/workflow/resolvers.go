package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/mcp"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/engine/tool/native"
	"github.com/compozy/compozy/pkg/logger"
)

var (
	jaroWinklerPool = sync.Pool{New: func() any { return metrics.NewJaroWinkler() }}
	levenshteinPool = sync.Pool{New: func() any { return metrics.NewLevenshtein() }}
)

func resolveAgent(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *agent.Config,
) (*agent.Config, error) {
	if isAgentSelector(in) {
		return resolveAgentBySelector(ctx, proj, store, in)
	}
	return buildInlineAgent(ctx, proj, store, in)
}

func resolveAgentBySelector(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *agent.Config,
) (*agent.Config, error) {
	log := logger.FromContext(ctx)
	key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceAgent, ID: in.ID}
	val, _, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, &SelectorNotFoundError{
				Type:       resources.ResourceAgent,
				ID:         in.ID,
				Project:    proj.Name,
				Candidates: nearestIDs(ctx, store, proj.Name, resources.ResourceAgent, in.ID),
			}
		}
		return nil, fmt.Errorf("agent lookup failed for '%s': %w", in.ID, err)
	}
	got, err := agentConfigFromStore(val)
	if err != nil {
		return nil, fmt.Errorf("agent decode failed for '%s': %w", in.ID, err)
	}
	if got == nil {
		return nil, &TypeMismatchError{Type: resources.ResourceAgent, ID: in.ID, Got: val}
	}
	clone, err := got.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone agent '%s': %w", in.ID, err)
	}
	if err := finishAgentSetup(ctx, proj, store, clone, in); err != nil {
		return nil, err
	}
	log.Debug("Resolved agent selector", "agent_id", in.ID)
	return clone, nil
}

func buildInlineAgent(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *agent.Config,
) (*agent.Config, error) {
	clone, err := in.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone inline agent '%s': %w", in.ID, err)
	}
	if err := finishAgentSetup(ctx, proj, store, clone, in); err != nil {
		return nil, err
	}
	return clone, nil
}

func finishAgentSetup(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	agentCfg *agent.Config,
	modelRefProvider *agent.Config,
) error {
	ensureAgentDefaults(agentCfg)
	if modelRefProvider != nil && modelRefProvider.Model.HasRef() {
		if err := applyAgentModelSelector(ctx, proj, store, agentCfg, modelRefProvider.Model.Ref); err != nil {
			return err
		}
	}
	proj.SetDefaultModel(agentCfg)
	if err := linkAgentSchemas(ctx, proj, store, agentCfg); err != nil {
		return err
	}
	// Resolve MCP selectors declared on the agent (e.g., { id: "srv" }).
	if err := resolveMCPs(ctx, proj, store, agentCfg); err != nil {
		return err
	}
	return nil
}

func agentConfigFromStore(value any) (*agent.Config, error) {
	switch tv := value.(type) {
	case *agent.Config:
		ensureAgentDefaults(tv)
		return tv, nil
	case map[string]any:
		cfg := &agent.Config{}
		if err := cfg.FromMap(tv); err != nil {
			return nil, err
		}
		ensureAgentDefaults(cfg)
		return cfg, nil
	case agent.Config:
		ensureAgentDefaults(&tv)
		return &tv, nil
	default:
		return nil, nil
	}
}

func toolConfigFromStore(value any) (*tool.Config, error) {
	return configFromStore(value, ensureToolDefaults)
}

func mcpConfigFromStore(value any) (*mcp.Config, error) {
	return configFromStore(value, func(cfg *mcp.Config) {
		cfg.SetDefaults()
	})
}

func modelConfigFromStore(value any) (*core.ProviderConfig, error) {
	return configFromStore(value, ensureProviderDefaults)
}

func configFromStore[T any](value any, mapNormalizer func(*T)) (*T, error) {
	switch tv := value.(type) {
	case *T:
		return tv, nil
	case map[string]any:
		cfg, err := core.FromMapDefault[*T](tv)
		if err != nil {
			return nil, err
		}
		if mapNormalizer != nil {
			mapNormalizer(cfg)
		}
		return cfg, nil
	case T:
		tmp := tv
		return &tmp, nil
	default:
		return nil, nil
	}
}

func ensureAgentDefaults(cfg *agent.Config) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.Resource) == "" {
		cfg.Resource = string(core.ConfigAgent)
	}
}

func ensureToolDefaults(cfg *tool.Config) {
	if cfg == nil {
		return
	}
	if strings.TrimSpace(cfg.Resource) == "" {
		cfg.Resource = string(core.ConfigTool)
	}
}

func newBuiltinToolConfig(id string) (*tool.Config, error) {
	def, ok := native.DefinitionByID(id)
	if !ok {
		return nil, nil
	}
	cfg := &tool.Config{
		Resource:    string(core.ConfigTool),
		ID:          def.ID,
		Description: def.Description,
	}
	if def.InputSchema != nil {
		clone, err := def.InputSchema.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone builtin input schema for '%s': %w", id, err)
		}
		cfg.InputSchema = clone
	}
	if def.OutputSchema != nil {
		clone, err := def.OutputSchema.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone builtin output schema for '%s': %w", id, err)
		}
		cfg.OutputSchema = clone
	}
	return cfg, nil
}

func ensureProviderDefaults(cfg *core.ProviderConfig) {
	if cfg == nil {
		return
	}
	cfg.Provider = core.ProviderName(strings.TrimSpace(string(cfg.Provider)))
	cfg.Model = strings.TrimSpace(cfg.Model)
	cfg.APIKey = strings.TrimSpace(cfg.APIKey)
	cfg.APIURL = strings.TrimSpace(cfg.APIURL)
	cfg.Organization = strings.TrimSpace(cfg.Organization)
}

func resolveTool(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *tool.Config,
) (*tool.Config, error) {
	log := logger.FromContext(ctx)
	if isToolSelector(in) {
		if builtinCfg, err := newBuiltinToolConfig(in.ID); err != nil {
			return nil, err
		} else if builtinCfg != nil {
			log.Debug("Resolved builtin tool selector", "tool_id", in.ID)
			return builtinCfg, nil
		}
		key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceTool, ID: in.ID}
		val, _, err := store.Get(ctx, key)
		if err != nil {
			if errors.Is(err, resources.ErrNotFound) {
				return nil, &SelectorNotFoundError{
					Type:       resources.ResourceTool,
					ID:         in.ID,
					Project:    proj.Name,
					Candidates: nearestIDs(ctx, store, proj.Name, resources.ResourceTool, in.ID),
				}
			}
			return nil, fmt.Errorf("tool lookup failed for '%s': %w", in.ID, err)
		}
		got, err := toolConfigFromStore(val)
		if err != nil {
			return nil, fmt.Errorf("tool decode failed for '%s': %w", in.ID, err)
		}
		if got == nil {
			return nil, &TypeMismatchError{Type: resources.ResourceTool, ID: in.ID, Got: val}
		}
		clone, err := got.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone tool '%s': %w", in.ID, err)
		}
		ensureToolDefaults(clone)
		if err := linkToolSchemas(ctx, proj, store, clone); err != nil {
			return nil, err
		}
		log.Debug("Resolved tool selector", "tool_id", in.ID)
		return clone, nil
	}
	clone, err := in.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone inline tool '%s': %w", in.ID, err)
	}
	ensureToolDefaults(clone)
	if err := linkToolSchemas(ctx, proj, store, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// linkWorkflowSchemas resolves schema ID references in workflow config input and triggers.
func linkWorkflowSchemas(ctx context.Context, proj *project.Config, store resources.ResourceStore, w *Config) error {
	if w == nil {
		return nil
	}
	if err := linkWorkflowInputSchema(ctx, proj, store, w); err != nil {
		return err
	}
	if err := linkWorkflowTriggersSchemas(ctx, proj, store, w); err != nil {
		return err
	}
	return nil
}

func linkWorkflowInputSchema(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	w *Config,
) error {
	ctxIn := fmt.Sprintf("workflow '%s' input", w.ID)
	if err := resolveSchemaRef(ctx, proj, store, &w.Opts.InputSchema, ctxIn); err != nil {
		return err
	}
	return nil
}

func linkWorkflowTriggersSchemas(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	w *Config,
) error {
	for i := range w.Triggers {
		t := &w.Triggers[i]
		ctxTrig := fmt.Sprintf("trigger '%s'", t.Name)
		if err := resolveSchemaRef(ctx, proj, store, &t.Schema, ctxTrig); err != nil {
			return err
		}
		if t.Webhook != nil {
			for ei := range t.Webhook.Events {
				e := &t.Webhook.Events[ei]
				ctxEvt := fmt.Sprintf("webhook event '%s'", e.Name)
				if err := resolveSchemaRef(ctx, proj, store, &e.Schema, ctxEvt); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// linkTaskSchemas resolves schema ID references on task-level input/output.
func linkTaskSchemas(ctx context.Context, proj *project.Config, store resources.ResourceStore, t *task.Config) error {
	if t == nil {
		return nil
	}
	if err := resolveSchemaRef(ctx, proj, store, &t.InputSchema, fmt.Sprintf("task '%s' input", t.ID)); err != nil {
		return err
	}
	if err := resolveSchemaRef(ctx, proj, store, &t.OutputSchema, fmt.Sprintf("task '%s' output", t.ID)); err != nil {
		return err
	}
	return nil
}

// linkToolSchemas resolves schema ID references on tool input/output.
func linkToolSchemas(ctx context.Context, proj *project.Config, store resources.ResourceStore, tl *tool.Config) error {
	if tl == nil {
		return nil
	}
	if err := resolveSchemaRef(ctx, proj, store, &tl.InputSchema, fmt.Sprintf("tool '%s' input", tl.ID)); err != nil {
		return err
	}
	if err := resolveSchemaRef(ctx, proj, store, &tl.OutputSchema, fmt.Sprintf("tool '%s' output", tl.ID)); err != nil {
		return err
	}
	return nil
}

// linkAgentSchemas resolves schema ID references on agent actions.
func linkAgentSchemas(ctx context.Context, proj *project.Config, store resources.ResourceStore, a *agent.Config) error {
	if a == nil {
		return nil
	}
	for i := range a.Actions {
		ac := a.Actions[i]
		if ac == nil {
			continue
		}
		ctxIn := fmt.Sprintf("agent '%s' action '%s' input", a.ID, ac.ID)
		if err := resolveSchemaRef(ctx, proj, store, &ac.InputSchema, ctxIn); err != nil {
			return err
		}
		ctxOut := fmt.Sprintf("agent '%s' action '%s' output", a.ID, ac.ID)
		if err := resolveSchemaRef(ctx, proj, store, &ac.OutputSchema, ctxOut); err != nil {
			return err
		}
	}
	return nil
}

// fetchSchema loads a schema by ID from the ResourceStore and returns a deep copy.
func fetchSchema(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	id string,
) (*schema.Schema, error) {
	key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceSchema, ID: id}
	val, _, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return nil, &SelectorNotFoundError{
				Type:       resources.ResourceSchema,
				ID:         id,
				Project:    proj.Name,
				Candidates: nearestIDs(ctx, store, proj.Name, resources.ResourceSchema, id),
			}
		}
		return nil, fmt.Errorf("schema lookup failed for '%s': %w", id, err)
	}
	sc, err := schemaFromStore(val)
	if err != nil {
		return nil, fmt.Errorf("schema decode failed for '%s': %w", id, err)
	}
	if sc == nil {
		return nil, &TypeMismatchError{Type: resources.ResourceSchema, ID: id, Got: val}
	}
	clone, err := sc.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone schema '%s': %w", id, err)
	}
	return clone, nil
}

func schemaFromStore(value any) (*schema.Schema, error) {
	switch tv := value.(type) {
	case *schema.Schema:
		return tv, nil
	case schema.Schema:
		tmp := tv
		return &tmp, nil
	case map[string]any:
		return core.FromMapDefault[*schema.Schema](tv)
	default:
		return nil, nil
	}
}

func hasAgentInlineContent(a *agent.Config) bool {
	if a == nil {
		return false
	}
	if strings.TrimSpace(a.Instructions) != "" {
		return true
	}
	if len(a.Actions) > 0 {
		return true
	}
	if !a.Model.IsEmpty() {
		return true
	}
	if a.Tools != nil {
		return true
	}
	if a.MCPs != nil {
		return true
	}
	if a.Memory != nil {
		return true
	}
	if len(a.Attachments) > 0 {
		return true
	}
	if a.Env != nil {
		return true
	}
	if a.MaxIterations != 0 {
		return true
	}
	return false
}

func hasToolInlineContent(t *tool.Config) bool {
	if t == nil {
		return false
	}
	if strings.TrimSpace(t.Description) != "" {
		return true
	}
	if strings.TrimSpace(t.Timeout) != "" {
		return true
	}
	if t.InputSchema != nil {
		return true
	}
	if t.OutputSchema != nil {
		return true
	}
	if t.With != nil {
		return true
	}
	if t.Config != nil {
		return true
	}
	if t.Env != nil {
		return true
	}
	if t.CWD != nil {
		return true
	}
	return false
}

func hasMCPInlineContent(mc *mcp.Config) bool {
	if mc == nil {
		return false
	}
	if strings.TrimSpace(mc.URL) != "" {
		return true
	}
	if strings.TrimSpace(mc.Command) != "" {
		return true
	}
	if mc.Headers != nil {
		return true
	}
	if mc.Env != nil {
		return true
	}
	if strings.TrimSpace(mc.Proto) != "" {
		return true
	}
	if mc.Transport != "" {
		return true
	}
	if mc.StartTimeout != 0 {
		return true
	}
	if mc.MaxSessions != 0 {
		return true
	}
	return false
}

func resolveSchemaRef(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	schemaPtr **schema.Schema,
	errContext string,
) error {
	if schemaPtr == nil || *schemaPtr == nil {
		return nil
	}
	if isRef, id := (*schemaPtr).IsRef(); isRef {
		sc, err := fetchSchema(ctx, proj, store, id)
		if err != nil {
			return fmt.Errorf("%s schema '%s' lookup failed: %w", errContext, id, err)
		}
		*schemaPtr = sc
	}
	return nil
}

func isAgentSelector(a *agent.Config) bool {
	if a == nil {
		return false
	}
	if strings.TrimSpace(a.ID) == "" {
		return false
	}
	return !hasAgentInlineContent(a)
}

func isToolSelector(t *tool.Config) bool {
	if t == nil {
		return false
	}
	if strings.TrimSpace(t.ID) == "" {
		return false
	}
	return !hasToolInlineContent(t)
}

// isMCPSelector returns true when an MCP config is an ID-only selector.
// We detect selectors using the absence of substantive configuration fields.
func isMCPSelector(mc *mcp.Config) bool {
	if mc == nil {
		return false
	}
	if strings.TrimSpace(mc.ID) == "" {
		return false
	}
	return !hasMCPInlineContent(mc)
}

// resolveMCPs resolves any ID-only MCP selectors on the provided agent config
// by fetching the concrete definitions from the ResourceStore. Resolved entries
// are deep-copied, defaults are applied, and basic validation is performed to
// surface configuration errors early in the compile phase.
func resolveMCPs(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	a *agent.Config,
) error {
	if a == nil || len(a.MCPs) == 0 {
		return nil
	}
	log := logger.FromContext(ctx)
	for i := range a.MCPs {
		if !isMCPSelector(&a.MCPs[i]) {
			// Inline definition: ensure defaults for consistency
			a.MCPs[i].SetDefaults()
			continue
		}
		id := a.MCPs[i].ID
		key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceMCP, ID: id}
		val, _, err := store.Get(ctx, key)
		if err != nil {
			if errors.Is(err, resources.ErrNotFound) {
				return &SelectorNotFoundError{
					Type:       resources.ResourceMCP,
					ID:         id,
					Project:    proj.Name,
					Candidates: nearestIDs(ctx, store, proj.Name, resources.ResourceMCP, id),
				}
			}
			return fmt.Errorf("mcp lookup failed for '%s': %w", id, err)
		}
		got, err := mcpConfigFromStore(val)
		if err != nil {
			return fmt.Errorf("mcp decode failed for '%s': %w", id, err)
		}
		if got == nil {
			return &TypeMismatchError{Type: resources.ResourceMCP, ID: id, Got: val}
		}
		clone, err := got.Clone()
		if err != nil {
			return fmt.Errorf("failed to clone mcp '%s': %w", id, err)
		}
		// Apply defaults; defer validation to explicit validation phases to
		// keep compile independent from runtime environment requirements.
		clone.SetDefaults()
		a.MCPs[i] = *clone
		log.Debug("Resolved mcp selector", "mcp_id", id)
	}
	return nil
}

// applyAgentModelSelector resolves a model resource by ID and merges it into the
// agent's ProviderConfig, preserving any explicitly set fields on the agent.
func applyAgentModelSelector(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	a *agent.Config,
	modelID string,
) error {
	key := resources.ResourceKey{Project: proj.Name, Type: resources.ResourceModel, ID: modelID}
	val, _, err := store.Get(ctx, key)
	if err != nil {
		if errors.Is(err, resources.ErrNotFound) {
			return &SelectorNotFoundError{
				Type:       resources.ResourceModel,
				ID:         modelID,
				Project:    proj.Name,
				Candidates: nearestIDs(ctx, store, proj.Name, resources.ResourceModel, modelID),
			}
		}
		return fmt.Errorf("model lookup failed for '%s': %w", modelID, err)
	}
	// Models are stored as *core.ProviderConfig
	pc, err := modelConfigFromStore(val)
	if err != nil {
		return fmt.Errorf("model decode failed for '%s': %w", modelID, err)
	}
	if pc == nil {
		return &TypeMismatchError{Type: resources.ResourceModel, ID: modelID, Got: val}
	}
	// Task-level/agent-level model selector takes precedence over defaults per PRD
	// Override identity (Provider/Model) from the referenced model; fill other fields when empty.
	overrideProviderIdentity(&a.Model.Config, pc)
	mergeProviderParamsPreferDst(&a.Model.Config.Params, &pc.Params)
	if a.Model.Config.APIKey == "" {
		a.Model.Config.APIKey = pc.APIKey
	}
	if a.Model.Config.APIURL == "" {
		a.Model.Config.APIURL = pc.APIURL
	}
	if a.Model.Config.Organization == "" {
		a.Model.Config.Organization = pc.Organization
	}
	if a.Model.Config.MaxToolIterations == 0 {
		a.Model.Config.MaxToolIterations = pc.MaxToolIterations
	}
	return nil
}

// overrideProviderIdentity sets Provider and Model from src unconditionally.
func overrideProviderIdentity(dst *core.ProviderConfig, src *core.ProviderConfig) {
	if dst == nil || src == nil {
		return
	}
	dst.Provider = src.Provider
	dst.Model = src.Model
}

// mergeProviderParamsPreferDst copies unset params from src; keeps dst values if already set.
func mergeProviderParamsPreferDst(dst *core.PromptParams, src *core.PromptParams) {
	if dst == nil || src == nil {
		return
	}
	mergeTokensAndTemperature(dst, src)
	mergeStopWords(dst, src)
	mergeSamplingParams(dst, src)
	mergeAdvancedParams(dst, src)
}

func mergeTokensAndTemperature(dst *core.PromptParams, src *core.PromptParams) {
	if !dst.IsSetMaxTokens() && src.IsSetMaxTokens() {
		dst.MaxTokens = src.MaxTokens
	}
	if !dst.IsSetTemperature() && src.IsSetTemperature() {
		dst.Temperature = src.Temperature
	}
}

func mergeStopWords(dst *core.PromptParams, src *core.PromptParams) {
	if !dst.IsSetStopWords() && src.IsSetStopWords() && len(src.StopWords) > 0 {
		dst.StopWords = append([]string(nil), src.StopWords...)
	}
}

func mergeSamplingParams(dst *core.PromptParams, src *core.PromptParams) {
	if !dst.IsSetTopK() && src.IsSetTopK() {
		dst.TopK = src.TopK
	}
	if !dst.IsSetTopP() && src.IsSetTopP() {
		dst.TopP = src.TopP
	}
}

func mergeAdvancedParams(dst *core.PromptParams, src *core.PromptParams) {
	if !dst.IsSetSeed() && src.IsSetSeed() {
		dst.Seed = src.Seed
	}
	if !dst.IsSetMinLength() && src.IsSetMinLength() {
		dst.MinLength = src.MinLength
	}
	if !dst.IsSetRepetitionPenalty() && src.IsSetRepetitionPenalty() {
		dst.RepetitionPenalty = src.RepetitionPenalty
	}
}

// nearestIDs returns up to 5 nearest IDs by prefix and edit distance within a project/type.
func nearestIDs(
	ctx context.Context,
	store resources.ResourceStore,
	project string,
	typ resources.ResourceType,
	target string,
) []string {
	log := logger.FromContext(ctx)
	keys, err := store.List(ctx, project, typ)
	if err != nil {
		log.Debug("list for suggestions failed", "project", project, "type", string(typ), "err", err)
		return nil
	}
	ids := make([]string, len(keys))
	for i := range keys {
		ids[i] = keys[i].ID
	}
	const suggestionLimit = 5
	return rankAndSelectTopIDs(target, ids, suggestionLimit)
}

func rankAndSelectTopIDs(target string, ids []string, limit int) []string {
	type cand struct {
		id     string
		prefix bool
		jwsim  float64
		lev    int
	}
	cands := make([]cand, 0, len(ids))
	lower := strings.ToLower(target)
	jwVal := jaroWinklerPool.Get()
	jw, ok := jwVal.(*metrics.JaroWinkler)
	if !ok || jw == nil {
		jw = metrics.NewJaroWinkler()
	}
	defer jaroWinklerPool.Put(jw)
	levVal := levenshteinPool.Get()
	levm, ok := levVal.(*metrics.Levenshtein)
	if !ok || levm == nil {
		levm = metrics.NewLevenshtein()
	}
	defer levenshteinPool.Put(levm)
	for _, id := range ids {
		s := strings.ToLower(id)
		c := cand{id: id}
		if strings.HasPrefix(s, lower) {
			c.prefix = true
		}
		c.jwsim = strutil.Similarity(lower, s, jw)
		c.lev = levm.Distance(lower, s)
		cands = append(cands, c)
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].prefix != cands[j].prefix {
			return cands[i].prefix && !cands[j].prefix
		}
		if cands[i].jwsim != cands[j].jwsim {
			return cands[i].jwsim > cands[j].jwsim
		}
		if cands[i].lev != cands[j].lev {
			return cands[i].lev < cands[j].lev
		}
		return cands[i].id < cands[j].id
	})
	if len(cands) < limit {
		limit = len(cands)
	}
	out := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		out = append(out, cands[i].id)
	}
	return out
}
