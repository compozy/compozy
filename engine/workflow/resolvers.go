package workflow

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/schema"
	"github.com/compozy/compozy/engine/task"
	"github.com/compozy/compozy/engine/tool"
	"github.com/compozy/compozy/pkg/logger"
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
	got, ok := val.(*agent.Config)
	if !ok {
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
	if modelRefProvider != nil && modelRefProvider.Model.HasRef() {
		if err := applyAgentModelSelector(ctx, proj, store, agentCfg, modelRefProvider.Model.Ref); err != nil {
			return err
		}
	}
	proj.SetDefaultModel(agentCfg)
	if err := linkAgentSchemas(ctx, proj, store, agentCfg); err != nil {
		return err
	}
	return nil
}

func resolveTool(
	ctx context.Context,
	proj *project.Config,
	store resources.ResourceStore,
	in *tool.Config,
) (*tool.Config, error) {
	log := logger.FromContext(ctx)
	if isToolSelector(in) {
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
		got, ok := val.(*tool.Config)
		if !ok {
			return nil, &TypeMismatchError{Type: resources.ResourceTool, ID: in.ID, Got: val}
		}
		clone, err := got.Clone()
		if err != nil {
			return nil, fmt.Errorf("failed to clone tool '%s': %w", in.ID, err)
		}
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
	sc, ok := val.(*schema.Schema)
	if !ok {
		return nil, &TypeMismatchError{Type: resources.ResourceSchema, ID: id, Got: val}
	}
	clone, err := sc.Clone()
	if err != nil {
		return nil, fmt.Errorf("failed to clone schema '%s': %w", id, err)
	}
	return clone, nil
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
	hasID := a.ID != ""
	noModelCfg := a.Model.Config.Provider == "" && a.Model.Config.Model == "" && !a.Model.HasRef()
	noInstr := a.Instructions == ""
	return hasID && noModelCfg && noInstr && len(a.Tools) == 0 && len(a.MCPs) == 0
}

func isToolSelector(t *tool.Config) bool {
	if t == nil {
		return false
	}
	return t.ID != "" && t.Description == "" && t.Timeout == "" && t.InputSchema == nil && t.OutputSchema == nil
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
	pc, ok := val.(*core.ProviderConfig)
	if !ok {
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
	ids := make([]string, 0, len(keys))
	for i := range keys {
		ids = append(ids, keys[i].ID)
	}
	const suggestionLimit = 5
	return rankAndSelectTopIDs(target, ids, suggestionLimit)
}

func rankAndSelectTopIDs(target string, ids []string, limit int) []string {
	type cand struct {
		id    string
		score int
	}
	cands := make([]cand, 0, len(ids))
	lower := strings.ToLower(target)
	for _, id := range ids {
		s := strings.ToLower(id)
		if strings.HasPrefix(s, lower) {
			cands = append(cands, cand{id: id, score: 0})
			continue
		}
		cands = append(cands, cand{id: id, score: levenshtein(lower, s)})
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score == cands[j].score {
			return cands[i].id < cands[j].id
		}
		return cands[i].score < cands[j].score
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

// levenshtein computes Levenshtein edit distance between a and b.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	la := len(a)
	lb := len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	curr := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		curr[0] = i
		ai := a[i-1]
		for j := 1; j <= lb; j++ {
			cost := 0
			if ai != b[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := curr[j-1] + 1
			sub := prev[j-1] + cost
			m := del
			if ins < m {
				m = ins
			}
			if sub < m {
				m = sub
			}
			curr[j] = m
		}
		prev, curr = curr, prev
	}
	return prev[lb]
}
