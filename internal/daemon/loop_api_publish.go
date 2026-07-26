package daemon

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	core "github.com/compozy/agh/internal/api/core"
	aghconfig "github.com/compozy/agh/internal/config"
	looppkg "github.com/compozy/agh/internal/loop"
	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/resources"
)

func (s *daemonLoopAPIService) findLoopRecord(
	workspaceID string,
	name string,
) (looppkg.WorkspaceID, resources.Record[looppkg.ResourceSpec], error) {
	ws, err := normalizeLoopWorkspaceID(workspaceID)
	if err != nil {
		return "", resources.Record[looppkg.ResourceSpec]{}, err
	}
	loopName, err := looppkg.ValidateName(name)
	if err != nil {
		return "", resources.Record[looppkg.ResourceSpec]{}, fmt.Errorf("%w: %v", looppkg.ErrValidation, err)
	}
	records := looppkg.ResolveEffectiveResources(s.catalog.Snapshot(), string(ws))
	for _, record := range records {
		if strings.TrimSpace(record.Spec.Name) == loopName {
			return ws, record, nil
		}
	}
	return "", resources.Record[looppkg.ResourceSpec]{}, fmt.Errorf(
		"%w: loop definition %q not found",
		looppkg.ErrDefinitionNotFound,
		loopName,
	)
}

func (s *daemonLoopAPIService) workspaceLoopRoot(
	ctx context.Context,
	workspaceID looppkg.WorkspaceID,
) (string, error) {
	if s.workspaceResolver == nil {
		return "", errors.New("daemon: workspace resolver is required")
	}
	workspace, err := s.workspaceResolver.Get(ctx, string(workspaceID))
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(workspace.RootDir)
	if root == "" {
		return "", fmt.Errorf("%w: workspace root is required", looppkg.ErrValidation)
	}
	return filepath.Join(root, aghconfig.DirName, aghconfig.LoopsDirName), nil
}

func ensureWritableLoopSource(spec looppkg.ResourceSpec) error {
	if spec.Source.Normalize() != looppkg.SourceWorkspace {
		return fmt.Errorf(
			"%w: loop definition %q is read-only; fork it into the workspace first",
			looppkg.ErrValidation,
			spec.Name,
		)
	}
	return nil
}

func (s *daemonLoopAPIService) compileForPublish(ctx context.Context, def dsl.Definition) error {
	compiler := newLoopCompilerWithSchemaSource(newLoopToolSchemaSource(ctx, s.toolRegistry))
	if _, err := compiler.Compile(def); err != nil {
		if lint, ok := errors.AsType[*looppkg.LintFailedError](err); ok {
			return &core.LoopLintFailedError{Errors: loopLintErrorsPayload(lint.Errors)}
		}
		return err
	}
	return nil
}

func (s *daemonLoopAPIService) writeDefinition(
	ctx context.Context,
	root string,
	def dsl.Definition,
	overwrite bool,
) (string, error) {
	data, err := dsl.Serialize(def)
	if err != nil {
		return "", fmt.Errorf("%w: %v", looppkg.ErrValidation, err)
	}
	_, path, err := looppkg.WriteDefinition(root, data, looppkg.WriteDefinitionOptions{
		Source:    looppkg.SourceWorkspace,
		Overwrite: overwrite,
		Linter:    newLoopLinterWithSchemaSource(newLoopToolSchemaSource(ctx, s.toolRegistry)),
	})
	if err != nil {
		if lint, ok := errors.AsType[*looppkg.LintFailedError](err); ok {
			return "", &core.LoopLintFailedError{Errors: loopLintErrorsPayload(lint.Errors)}
		}
		return "", err
	}
	if err := s.syncLoopResources(ctx); err != nil {
		return "", err
	}
	return path, nil
}

func (s *daemonLoopAPIService) syncLoopResources(ctx context.Context) error {
	if s.publisher == nil {
		return nil
	}
	return s.publisher.Sync(ctx)
}

func (s *daemonLoopAPIService) loopResponseFromDefinitionFile(
	ctx context.Context,
	path string,
	workspaceID looppkg.WorkspaceID,
	name string,
) (contract.LoopResponse, error) {
	record, def, err := s.refreshCatalogLoopRecordFromFile(ctx, path, workspaceID, name)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	payload, err := loopDefinitionPayload(record.Spec, def)
	if err != nil {
		return contract.LoopResponse{}, err
	}
	return contract.LoopResponse{Loop: payload}, nil
}

func (s *daemonLoopAPIService) refreshCatalogLoopRecordFromFile(
	ctx context.Context,
	path string,
	workspaceID looppkg.WorkspaceID,
	name string,
) (resources.Record[looppkg.ResourceSpec], dsl.Definition, error) {
	spec, def, err := looppkg.ParseResourceFile(path, looppkg.ResourceParseOptions{
		Source: looppkg.SourceWorkspace,
		Linter: newLoopLinterWithSchemaSource(newLoopToolSchemaSource(ctx, s.toolRegistry)),
	})
	if err != nil {
		return resources.Record[looppkg.ResourceSpec]{}, dsl.Definition{}, err
	}
	loopName, err := looppkg.ValidateName(name)
	if err != nil {
		return resources.Record[looppkg.ResourceSpec]{}, dsl.Definition{}, fmt.Errorf(
			"%w: %v",
			looppkg.ErrValidation,
			err,
		)
	}
	if strings.TrimSpace(spec.Name) != loopName {
		return resources.Record[looppkg.ResourceSpec]{}, dsl.Definition{}, fmt.Errorf(
			"%w: published definition name %q does not match %q",
			looppkg.ErrValidation,
			spec.Name,
			loopName,
		)
	}
	scope := resources.ResourceScope{Kind: resources.ResourceScopeKindWorkspace, ID: string(workspaceID)}
	record := resources.Record[looppkg.ResourceSpec]{
		ID:      "daemon.api.loop." + string(workspaceID) + "." + loopName,
		Version: int64(spec.Version),
		Scope:   scope,
		Spec:    spec,
	}
	s.upsertPublishedLoopRecord(record)
	return record, def, nil
}

func (s *daemonLoopAPIService) upsertPublishedLoopRecord(record resources.Record[looppkg.ResourceSpec]) {
	if s.catalog == nil {
		return
	}
	scope := record.Scope.Normalize()
	name := strings.TrimSpace(record.Spec.Name)
	source := record.Spec.Source.Normalize()
	s.catalog.Update(func(snapshot []resources.Record[looppkg.ResourceSpec]) []resources.Record[looppkg.ResourceSpec] {
		records := make([]resources.Record[looppkg.ResourceSpec], 0, len(snapshot)+1)
		for _, existing := range snapshot {
			existingScope := existing.Scope.Normalize()
			if existingScope.Kind == scope.Kind &&
				existingScope.ID == scope.ID &&
				strings.TrimSpace(existing.Spec.Name) == name &&
				existing.Spec.Source.Normalize() == source {
				continue
			}
			records = append(records, existing)
		}
		records = append(records, record)
		return records
	})
}
