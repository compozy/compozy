package daemon

import (
	"context"
	"errors"
	"net/http"
	"strings"

	apicore "github.com/compozy/compozy/internal/api/core"
	corepkg "github.com/compozy/compozy/internal/core"
	extensions "github.com/compozy/compozy/internal/core/extension"
	"github.com/compozy/compozy/internal/core/provider"
	"github.com/compozy/compozy/internal/core/providerdefaults"
	workspacecfg "github.com/compozy/compozy/internal/core/workspace"
	"github.com/compozy/compozy/internal/store/globaldb"
)

type transportReviewService struct {
	globalDB   *globaldb.GlobalDB
	runManager *RunManager
	query      QueryService
}

type transportExecService struct {
	runManager *RunManager
}

var _ apicore.ReviewService = (*transportReviewService)(nil)
var _ apicore.ExecService = (*transportExecService)(nil)

func newTransportReviewService(
	globalDB *globaldb.GlobalDB,
	runManager *RunManager,
	query ...QueryService,
) *transportReviewService {
	return &transportReviewService{
		globalDB:   globalDB,
		runManager: runManager,
		query:      resolveTransportQueryService(globalDB, runManager, nil, query),
	}
}

func newTransportExecService(runManager *RunManager) *transportExecService {
	return &transportExecService{runManager: runManager}
}

func (s *transportReviewService) Fetch(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.ReviewFetchRequest,
) (apicore.ReviewFetchResult, error) {
	if s == nil || s.globalDB == nil || s.runManager == nil {
		return apicore.ReviewFetchResult{}, reviewTransportUnavailable("review fetch")
	}

	workspaceRow, _, projectCfg, err := s.runManager.resolveWorkflowContext(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.ReviewFetchResult{}, err
	}

	registry, cleanup, err := buildWorkspaceReviewRegistry(ctx, workspaceRow.RootDir, "compozy reviews fetch")
	if err != nil {
		return apicore.ReviewFetchResult{}, err
	}
	defer cleanup()

	fetchCfg := reviewFetchConfig(workspaceRow.RootDir, workflowSlug, projectCfg, req)

	result, err := corepkg.FetchReviewsWithRegistryDirect(ctx, fetchCfg, registry)
	if err != nil {
		return apicore.ReviewFetchResult{}, err
	}
	roundRow, err := s.syncFetchedReviewRound(ctx, workspaceRow, workflowSlug, result)
	if err != nil {
		return apicore.ReviewFetchResult{}, err
	}
	return apicore.ReviewFetchResult{
		Summary: transportReviewSummary(strings.TrimSpace(workflowSlug), roundRow),
		Created: true,
	}, nil
}

func reviewFetchConfig(
	workspaceRoot string,
	workflowSlug string,
	projectCfg workspacecfg.ProjectConfig,
	req apicore.ReviewFetchRequest,
) corepkg.Config {
	fetchCfg := corepkg.Config{
		WorkspaceRoot: workspaceRoot,
		Name:          strings.TrimSpace(workflowSlug),
		Provider:      resolveFetchProvider(projectCfg, req.Provider),
		PR:            strings.TrimSpace(req.PRRef),
		Nitpicks:      resolveFetchNitpicks(projectCfg),
	}
	if req.Round != nil {
		fetchCfg.Round = *req.Round
	}
	return fetchCfg
}

func (s *transportReviewService) syncFetchedReviewRound(
	ctx context.Context,
	workspaceRow globaldb.Workspace,
	workflowSlug string,
	result *corepkg.FetchResult,
) (globaldb.ReviewRound, error) {
	slug := strings.TrimSpace(workflowSlug)
	syncResult, err := corepkg.SyncWithDB(ctx, s.globalDB, workspaceRow, corepkg.SyncConfig{
		WorkspaceRoot: workspaceRow.RootDir,
		Name:          slug,
	})
	if err != nil {
		return globaldb.ReviewRound{}, reviewFetchPostWriteProblem(
			"review_fetch_sync_failed",
			"review issues were fetched, but catalog sync failed",
			slug,
			result.Round,
			result.ReviewsDir,
			err,
		)
	}

	syncedWorkflow, err := s.globalDB.GetActiveWorkflowBySlug(ctx, workspaceRow.ID, slug)
	if err != nil {
		return globaldb.ReviewRound{}, reviewFetchPostWriteProblem(
			"review_fetch_round_lookup_failed",
			"review issues were fetched, but the synced workflow could not be reloaded",
			slug,
			result.Round,
			result.ReviewsDir,
			err,
		)
	}
	syncedWorkflowID := syncedWorkflow.ID
	s.runManager.publishWorkflowSyncWorkspaceEvent(
		ctx,
		workspaceRow.ID,
		&syncedWorkflowID,
		slug,
		syncResult.SyncedPaths,
	)

	roundRow, err := s.globalDB.GetReviewRound(ctx, syncedWorkflow.ID, result.Round)
	if err != nil {
		return globaldb.ReviewRound{}, reviewFetchPostWriteProblem(
			"review_fetch_round_lookup_failed",
			"review issues were fetched, but the synced review round could not be loaded",
			slug,
			result.Round,
			result.ReviewsDir,
			err,
		)
	}
	return roundRow, nil
}

func reviewFetchPostWriteProblem(
	code string,
	message string,
	workflowSlug string,
	round int,
	reviewsDir string,
	err error,
) error {
	return apicore.NewProblem(
		http.StatusInternalServerError,
		code,
		message,
		map[string]any{
			"workflow":    strings.TrimSpace(workflowSlug),
			"round":       round,
			"reviews_dir": strings.TrimSpace(reviewsDir),
		},
		err,
	)
}

func (s *transportReviewService) GetLatest(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (apicore.ReviewSummary, error) {
	if s == nil || s.globalDB == nil || s.runManager == nil {
		return apicore.ReviewSummary{}, reviewTransportUnavailable("review lookup")
	}

	workflow, err := s.resolveReviewWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.ReviewSummary{}, mapReviewLookupError(err)
	}
	row, err := s.globalDB.GetLatestReviewRound(ctx, workflow.ID)
	if err != nil {
		return apicore.ReviewSummary{}, mapReviewLookupError(err)
	}
	return transportReviewSummary(strings.TrimSpace(workflowSlug), row), nil
}

func (s *transportReviewService) GetRound(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
) (apicore.ReviewRound, error) {
	if s == nil || s.globalDB == nil || s.runManager == nil {
		return apicore.ReviewRound{}, reviewTransportUnavailable("review round lookup")
	}

	workflow, err := s.resolveReviewWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return apicore.ReviewRound{}, mapReviewLookupError(err)
	}
	row, err := s.globalDB.GetReviewRound(ctx, workflow.ID, round)
	if err != nil {
		return apicore.ReviewRound{}, mapReviewLookupError(err)
	}
	return transportReviewRound(strings.TrimSpace(workflowSlug), row), nil
}

func (s *transportReviewService) ListIssues(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
) ([]apicore.ReviewIssue, error) {
	if s == nil || s.globalDB == nil || s.runManager == nil {
		return nil, reviewTransportUnavailable("review issue listing")
	}

	workflow, err := s.resolveReviewWorkflow(ctx, workspaceRef, workflowSlug)
	if err != nil {
		return nil, mapReviewLookupError(err)
	}
	roundRow, err := s.globalDB.GetReviewRound(ctx, workflow.ID, round)
	if err != nil {
		return nil, mapReviewLookupError(err)
	}
	rows, err := s.globalDB.ListReviewIssues(ctx, roundRow.ID)
	if err != nil {
		return nil, err
	}
	issues := make([]apicore.ReviewIssue, 0, len(rows))
	for _, row := range rows {
		issues = append(issues, transportReviewIssue(row))
	}
	return issues, nil
}

func (s *transportReviewService) resolveReviewWorkflow(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
) (globaldb.Workflow, error) {
	workspaceRow, err := resolveWorkspaceReference(ctx, s.globalDB, workspaceRef)
	if err != nil {
		return globaldb.Workflow{}, err
	}
	workflow, err := s.globalDB.GetActiveWorkflowBySlug(ctx, workspaceRow.ID, workflowSlug)
	if err == nil {
		return workflow, nil
	}
	if !errors.Is(err, globaldb.ErrWorkflowNotFound) {
		return globaldb.Workflow{}, err
	}
	return s.globalDB.GetLatestArchivedWorkflowBySlug(ctx, workspaceRow.ID, workflowSlug)
}

func (s *transportReviewService) ReviewDetail(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
	issueRef string,
) (apicore.ReviewDetailPayload, error) {
	if s == nil || s.query == nil {
		return apicore.ReviewDetailPayload{}, reviewTransportUnavailable("review detail")
	}

	payload, err := s.query.ReviewDetail(ctx, workspaceRef, workflowSlug, round, issueRef)
	if err != nil {
		return apicore.ReviewDetailPayload{}, mapQueryTransportError(err)
	}
	return transportReviewDetail(payload), nil
}

func (s *transportReviewService) StartRun(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	round int,
	req apicore.ReviewRunRequest,
) (apicore.Run, error) {
	if s == nil || s.runManager == nil {
		return apicore.Run{}, reviewTransportUnavailable("review runs")
	}
	return s.runManager.StartReviewRun(ctx, workspaceRef, workflowSlug, round, req)
}

func (s *transportReviewService) StartWatch(
	ctx context.Context,
	workspaceRef string,
	workflowSlug string,
	req apicore.ReviewWatchRequest,
) (apicore.Run, error) {
	if s == nil || s.runManager == nil {
		return apicore.Run{}, reviewTransportUnavailable("review watch")
	}
	return s.runManager.StartReviewWatch(ctx, workspaceRef, workflowSlug, req)
}

func (s *transportExecService) Start(ctx context.Context, req apicore.ExecRequest) (apicore.Run, error) {
	if s == nil || s.runManager == nil {
		return apicore.Run{}, execTransportUnavailable()
	}
	return s.runManager.StartExecRun(ctx, req)
}

func buildWorkspaceReviewRegistry(
	ctx context.Context,
	workspaceRoot string,
	invokingCommand string,
) (provider.RegistryReader, func(), error) {
	discovery, err := extensions.Discovery{WorkspaceRoot: workspaceRoot}.Discover(ctx)
	if err != nil {
		return nil, nil, err
	}

	entries := make([]provider.OverlayEntry, 0, len(discovery.Providers.Review))
	bridges := make([]provider.ExtensionBridge, 0)
	for idx := range discovery.Providers.Review {
		entry := discovery.Providers.Review[idx]
		overlay := provider.OverlayEntry{
			Name:        entry.Name,
			DisplayName: entry.DisplayName,
			Command:     entry.Command,
			Target:      entry.Target,
			Kind:        provider.OverlayKind(entry.Kind),
			Metadata:    cloneStringMap(entry.Metadata),
		}
		if entry.Kind == extensions.ProviderKindExtension {
			bridge, err := extensions.NewReviewProviderBridge(entry, workspaceRoot, invokingCommand)
			if err != nil {
				closeOverlayBridges(bridges)
				return nil, nil, err
			}
			overlay.Bridge = bridge
			bridges = append(bridges, bridge)
		}
		entries = append(entries, overlay)
	}

	registry, err := provider.BuildOverlayRegistry(
		provider.ResolveRegistry(providerdefaults.DefaultRegistryForWorkspace(workspaceRoot)),
		entries,
	)
	if err != nil {
		closeOverlayBridges(bridges)
		return nil, nil, err
	}
	return registry, func() {
		closeOverlayBridges(bridges)
	}, nil
}

func closeOverlayBridges(bridges []provider.ExtensionBridge) {
	for _, bridge := range bridges {
		if bridge != nil {
			_ = bridge.Close()
		}
	}
}

func resolveFetchProvider(projectCfg workspacecfg.ProjectConfig, requested string) string {
	if providerName := strings.TrimSpace(requested); providerName != "" {
		return providerName
	}
	if projectCfg.FetchReviews.Provider == nil {
		return ""
	}
	return strings.TrimSpace(*projectCfg.FetchReviews.Provider)
}

func resolveFetchNitpicks(projectCfg workspacecfg.ProjectConfig) bool {
	if projectCfg.FetchReviews.Nitpicks != nil {
		return *projectCfg.FetchReviews.Nitpicks
	}
	return true
}

func mapReviewLookupError(err error) error {
	if errors.Is(err, globaldb.ErrReviewRoundNotFound) || errors.Is(err, globaldb.ErrWorkflowNotFound) {
		return apicore.NewProblem(
			http.StatusNotFound,
			"review_round_not_found",
			"review round was not found",
			nil,
			err,
		)
	}
	return err
}

func reviewTransportUnavailable(action string) error {
	return apicore.NewProblem(
		http.StatusServiceUnavailable,
		"review_service_unavailable",
		action+" is not available in this daemon build",
		nil,
		nil,
	)
}

func execTransportUnavailable() error {
	return apicore.NewProblem(
		http.StatusServiceUnavailable,
		"exec_service_unavailable",
		"exec runs are not available in this daemon build",
		nil,
		nil,
	)
}

func transportReviewSummary(workflowSlug string, row globaldb.ReviewRound) apicore.ReviewSummary {
	return apicore.ReviewSummary{
		WorkflowSlug:    workflowSlug,
		RoundNumber:     row.RoundNumber,
		Provider:        row.Provider,
		PRRef:           row.PRRef,
		ResolvedCount:   row.ResolvedCount,
		UnresolvedCount: row.UnresolvedCount,
		UpdatedAt:       row.UpdatedAt,
	}
}

func transportReviewRound(workflowSlug string, row globaldb.ReviewRound) apicore.ReviewRound {
	return apicore.ReviewRound{
		ID:              row.ID,
		WorkflowSlug:    workflowSlug,
		RoundNumber:     row.RoundNumber,
		Provider:        row.Provider,
		PRRef:           row.PRRef,
		ResolvedCount:   row.ResolvedCount,
		UnresolvedCount: row.UnresolvedCount,
		UpdatedAt:       row.UpdatedAt,
	}
}

func transportReviewIssue(row globaldb.ReviewIssue) apicore.ReviewIssue {
	return apicore.ReviewIssue{
		ID:          row.ID,
		IssueNumber: row.IssueNumber,
		Severity:    row.Severity,
		Status:      row.Status,
		SourcePath:  row.SourcePath,
		UpdatedAt:   row.UpdatedAt,
	}
}

func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return nil
	}
	dst := make(map[string]string, len(src))
	for key, value := range src {
		dst[key] = value
	}
	return dst
}
