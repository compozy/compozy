package extensionpkg

import (
	"cmp"
	"context"
	"encoding/json/v2"
	"errors"
	"slices"
	"strings"

	protocol "github.com/compozy/compozy/internal/extensionprotocol"
	"github.com/compozy/compozy/internal/heartbeat/source"
	"github.com/compozy/compozy/internal/subprocess"
)

var _ source.ExtensionPoller = (*Manager)(nil)

func (m *Manager) heartbeatSourceRuntime(
	ctx context.Context,
	scope source.CatalogScope,
	name string,
) (processHandle, []protocol.HeartbeatSourceKind, error) {
	key, err := m.ensureProfileRuntime(ctx, ProfileInstanceKey(name, scope.ProfileID, scope.WorkspaceID))
	if err != nil {
		return nil, nil, source.Failure("source_kind_unavailable", err.Error())
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	ext := m.instanceLocked(key)
	if ext == nil || !ext.active || ext.process == nil || ext.initialize == nil ||
		!slices.Contains(ext.initialize.AcceptedCapabilities.Provides, protocol.CapabilityProvideHeartbeatSource) ||
		!slices.Contains(
			ext.initialize.ImplementedMethods,
			string(protocol.ExtensionServiceMethodHeartbeatSourcePoll),
		) {
		return nil, nil, source.Failure("source_kind_unavailable", "extension is not enabled in this scope")
	}
	provides := ext.info.Capabilities.Provides
	if ext.manifest != nil {
		provides = ext.manifest.Capabilities.Provides
	}
	if !slices.Contains(provides, protocol.CapabilityProvideHeartbeatSource) {
		return nil, nil, source.Failure("source_kind_unavailable", "extension does not provide heartbeat.source")
	}
	kinds := slices.Clone(ext.initialize.HeartbeatSourceKinds)
	for i := range kinds {
		kinds[i].ParamsSchema = cloneRawMessage(kinds[i].ParamsSchema)
	}
	return ext.process, kinds, nil
}

func (m *Manager) SourceKinds(ctx context.Context, scope source.CatalogScope) ([]source.KindDescriptor, error) {
	if ctx == nil {
		return nil, ErrContextRequired
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m == nil {
		return nil, ErrManagerRequired
	}
	result := []source.KindDescriptor{}
	infos := m.ListForWorkspace(scope.WorkspaceID)
	for i := range infos {
		info := &infos[i]
		_, kinds, err := m.heartbeatSourceRuntime(ctx, scope, info.Name)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			continue
		}
		for _, kind := range kinds {
			result = append(
				result,
				source.KindDescriptor{
					Kind:         info.Name + "/" + kind.Kind,
					Extension:    info.Name,
					Title:        kind.Title,
					Description:  kind.Description,
					ParamsSchema: kind.ParamsSchema,
				},
			)
		}
	}
	slices.SortFunc(result, func(a, b source.KindDescriptor) int { return cmp.Compare(a.Kind, b.Kind) })
	return result, nil
}

func (m *Manager) PollSource(
	ctx context.Context,
	scope source.CatalogScope,
	ref string,
	req source.Request,
) (source.Result, error) {
	name, kind, ok := strings.Cut(ref, "/")
	if !ok || name == "" || kind == "" {
		return source.Result{}, source.Failure("source_kind_unavailable", "invalid extension kind")
	}
	process, kinds, err := m.heartbeatSourceRuntime(ctx, scope, name)
	if err != nil {
		return source.Result{}, err
	}
	index := slices.IndexFunc(kinds, func(k protocol.HeartbeatSourceKind) bool { return k.Kind == kind })
	if index < 0 {
		return source.Result{}, source.Failure("source_kind_unavailable", "kind is not registered")
	}
	params := req.Params
	if len(params) == 0 {
		params = []byte(`{}`)
	}
	if len(kinds[index].ParamsSchema) > 0 {
		schema, compileErr := compileCommandSchema(kinds[index].ParamsSchema)
		if compileErr != nil {
			return source.Result{}, source.Failure("source_params_invalid", compileErr.Error())
		}
		var value any
		if err := json.Unmarshal(params, &value); err != nil {
			return source.Result{}, source.Failure("source_params_invalid", err.Error())
		}
		if err := schema.Validate(value); err != nil {
			return source.Result{}, source.Failure("source_params_invalid", err.Error())
		}
	}
	req.Kind = kind
	req.Params = params
	var result source.Result
	if err := process.Call(ctx, string(protocol.ExtensionServiceMethodHeartbeatSourcePoll), req, &result); err != nil {
		code := "source_extension_failed"
		if errors.Is(err, context.DeadlineExceeded) {
			code = "source_timeout"
		}
		if rpc, ok := errors.AsType[*subprocess.RPCError](err); ok {
			var data struct {
				Code  string `json:"code"`
				Cause string `json:"cause"`
			}
			if json.Unmarshal(rpc.Data, &data) == nil {
				for _, cause := range []string{data.Code, data.Cause} {
					switch cause {
					case "rate_limited", "source_rate_limited":
						code = "source_rate_limited"
					case "source_credential_absent", "source_credential_expired", "source_params_invalid":
						code = cause
					}
				}
			}
		}
		return source.Result{}, source.Failure(code, err.Error())
	}
	if err := source.ValidateResult(result, req.Limits); err != nil {
		return source.Result{}, err
	}
	return result, nil
}
