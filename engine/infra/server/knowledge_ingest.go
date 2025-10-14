package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/compozy/compozy/engine/infra/server/appstate"
	"github.com/compozy/compozy/engine/knowledge"
	"github.com/compozy/compozy/engine/knowledge/ingest"
	"github.com/compozy/compozy/engine/knowledge/uc"
	"github.com/compozy/compozy/engine/project"
	"github.com/compozy/compozy/engine/resources"
	"github.com/compozy/compozy/engine/workflow"
	"github.com/compozy/compozy/pkg/config"
	"github.com/compozy/compozy/pkg/logger"
)

type startupIngestExecutor interface {
	Execute(context.Context, *uc.IngestInput) (*uc.IngestOutput, error)
}

var newStartupIngestExecutor = func(store resources.ResourceStore) startupIngestExecutor {
	return uc.NewIngest(store)
}

func ingestKnowledgeBasesOnStart(
	ctx context.Context,
	state *appstate.State,
	projectConfig *project.Config,
	workflows []*workflow.Config,
) error {
	if projectConfig == nil || state == nil {
		return nil
	}
	toIngest, err := collectStartupKnowledgeBases(projectConfig, workflows)
	if err != nil {
		return err
	}
	if len(toIngest) == 0 {
		return nil
	}
	storeVal, ok := state.ResourceStore()
	if !ok {
		return fmt.Errorf("knowledge: startup ingest requires a resource store")
	}
	store, ok := storeVal.(resources.ResourceStore)
	if !ok || store == nil {
		return fmt.Errorf("knowledge: startup ingest store has unexpected type %T", storeVal)
	}
	sort.Slice(toIngest, func(i, j int) bool { return toIngest[i].ID < toIngest[j].ID })
	cfg := config.FromContext(ctx)
	timeout := time.Duration(0)
	if cfg != nil {
		timeout = cfg.Server.Timeouts.KnowledgeIngest
	}
	ingestUseCase := newStartupIngestExecutor(store)
	for _, kb := range toIngest {
		if err := runStartupKnowledgeIngest(ctx, ingestUseCase, projectConfig, timeout, kb); err != nil {
			return err
		}
	}
	return nil
}

func runStartupKnowledgeIngest(
	ctx context.Context,
	ingestUseCase startupIngestExecutor,
	projectConfig *project.Config,
	timeout time.Duration,
	kb startupKnowledgeBase,
) error {
	log := logger.FromContext(ctx)
	log.Info(
		"startup knowledge ingestion triggered",
		"project", projectConfig.Name,
		"kb_id", kb.ID,
		"origin", kb.Origin,
	)
	runCtx := ctx
	cancel := func() {}
	if timeout > 0 {
		runCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	defer cancel()
	_, err := ingestUseCase.Execute(runCtx, &uc.IngestInput{
		Project:  projectConfig.Name,
		ID:       kb.ID,
		Strategy: ingest.StrategyReplace,
		CWD:      projectConfig.GetCWD(),
	})
	if err != nil {
		return fmt.Errorf("knowledge: startup ingest for %q failed: %w", kb.ID, err)
	}
	log.Info(
		"startup knowledge ingestion completed",
		"project", projectConfig.Name,
		"kb_id", kb.ID,
		"origin", kb.Origin,
	)
	return nil
}

type startupKnowledgeBase struct {
	ID     string
	Origin string
}

func collectStartupKnowledgeBases(
	projectConfig *project.Config,
	workflows []*workflow.Config,
) ([]startupKnowledgeBase, error) {
	if projectConfig == nil {
		return nil, fmt.Errorf("knowledge: project configuration is required for startup ingestion")
	}
	providers := make([]project.KnowledgeBaseProvider, 0, len(workflows))
	for _, wf := range workflows {
		if wf == nil {
			continue
		}
		providers = append(providers, wf)
	}
	refs := projectConfig.AggregatedKnowledgeBases(providers...)
	seen := make(map[string]string, len(refs))
	out := make([]startupKnowledgeBase, 0, len(refs))
	for i := range refs {
		ref := refs[i]
		id := strings.TrimSpace(ref.Base.ID)
		if id == "" {
			return nil, fmt.Errorf("knowledge: knowledge_base with empty id declared in %s", ref.Origin)
		}
		mode := ref.Base.Ingest
		if mode == "" {
			mode = knowledge.IngestManual
		}
		switch mode {
		case knowledge.IngestManual:
			continue
		case knowledge.IngestOnStart:
			if prev, exists := seen[id]; exists {
				return nil, fmt.Errorf(
					"knowledge: knowledge_base %q configured for startup ingestion in both %s and %s",
					id,
					prev,
					ref.Origin,
				)
			}
			seen[id] = ref.Origin
			out = append(out, startupKnowledgeBase{ID: id, Origin: ref.Origin})
		default:
			return nil, fmt.Errorf("knowledge: knowledge_base %q has unsupported ingest mode %q", id, mode)
		}
	}
	return out, nil
}
