package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	hookspkg "github.com/compozy/agh/internal/hooks"
	"github.com/compozy/agh/internal/resources"
)

const (
	hookBindingResourceKind     resources.ResourceKind = "hook.binding"
	hookBindingResourceMaxBytes                        = 256 << 10
)

type hookBindingProjectionPlan struct {
	revision   int64
	operations int
	state      *hookspkg.BindingState
}

func (p *hookBindingProjectionPlan) Kind() resources.ResourceKind {
	return hookBindingResourceKind
}

func (p *hookBindingProjectionPlan) Revision() int64 {
	if p == nil {
		return 0
	}
	return p.revision
}

func (p *hookBindingProjectionPlan) OperationCount() int {
	if p == nil {
		return 0
	}
	return p.operations
}

type hookBindingProjector struct {
	runtime *hookspkg.Hooks
}

type hookBindingCodec struct {
	inner resources.KindCodec[hookBindingCodecSpec]
}

type hookBindingCodecSpec struct {
	Name         string                    `json:"name"`
	Event        hookspkg.HookEvent        `json:"event"`
	Source       hookspkg.HookSource       `json:"source"`
	Mode         hookspkg.HookMode         `json:"mode,omitempty"`
	Required     bool                      `json:"required,omitempty"`
	Priority     int32                     `json:"priority,omitempty"`
	PrioritySet  bool                      `json:"priority_set,omitempty"`
	Timeout      time.Duration             `json:"timeout,omitempty"`
	Matcher      hookspkg.HookMatcher      `json:"matcher"`
	ExecutorKind hookspkg.HookExecutorKind `json:"executor_kind,omitempty"`
	Command      string                    `json:"command,omitempty"`
	Args         []string                  `json:"args,omitempty"`
	WorkingDir   string                    `json:"working_dir,omitempty"`
	Env          map[string]string         `json:"env,omitempty"`
	SecretEnv    map[string]string         `json:"secret_env,omitempty"`
	Metadata     map[string]string         `json:"metadata,omitempty"`
	SkillSource  hookspkg.HookSkillSource  `json:"skill_source,omitempty"`
}

var _ resources.TypedProjector[hookspkg.HookDecl] = (*hookBindingProjector)(nil)
var _ resources.KindCodec[hookspkg.HookDecl] = (*hookBindingCodec)(nil)

func newHookBindingCodec() (resources.KindCodec[hookspkg.HookDecl], error) {
	inner, err := resources.NewJSONCodec(
		hookBindingResourceKind,
		hookBindingResourceMaxBytes,
		func(
			ctx context.Context,
			scope resources.ResourceScope,
			spec hookBindingCodecSpec,
		) (hookBindingCodecSpec, error) {
			validated, err := validateHookBindingSpec(ctx, scope, spec.hookDecl())
			if err != nil {
				return hookBindingCodecSpec{}, err
			}
			return newHookBindingCodecSpec(validated), nil
		},
	)
	if err != nil {
		return nil, err
	}
	return &hookBindingCodec{inner: inner}, nil
}

func (c *hookBindingCodec) Kind() resources.ResourceKind {
	if c == nil || c.inner == nil {
		return hookBindingResourceKind
	}
	return c.inner.Kind()
}

func (c *hookBindingCodec) MaxBytes() int {
	if c == nil || c.inner == nil {
		return hookBindingResourceMaxBytes
	}
	return c.inner.MaxBytes()
}

func (c *hookBindingCodec) Encode(spec hookspkg.HookDecl) ([]byte, error) {
	if c == nil || c.inner == nil {
		return nil, errors.New("daemon: hook binding codec is required")
	}
	return c.inner.Encode(newHookBindingCodecSpec(spec))
}

func (c *hookBindingCodec) DecodeAndValidate(
	ctx context.Context,
	scope resources.ResourceScope,
	raw []byte,
) (hookspkg.HookDecl, error) {
	if c == nil || c.inner == nil {
		return hookspkg.HookDecl{}, errors.New("daemon: hook binding codec is required")
	}

	spec, err := c.inner.DecodeAndValidate(ctx, scope, raw)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}
	return spec.hookDecl(), nil
}

func (c *hookBindingCodec) ValidateAndCanonicalizeRaw(
	ctx context.Context,
	scope resources.ResourceScope,
	raw []byte,
) ([]byte, error) {
	if c == nil || c.inner == nil {
		return nil, errors.New("daemon: hook binding codec is required")
	}

	rawCodec, ok := c.inner.(interface {
		ValidateAndCanonicalizeRaw(context.Context, resources.ResourceScope, []byte) ([]byte, error)
	})
	if !ok {
		return nil, errors.New("daemon: hook binding codec does not support raw validation")
	}
	return rawCodec.ValidateAndCanonicalizeRaw(ctx, scope, raw)
}

func newHookBindingCodecSpec(decl hookspkg.HookDecl) hookBindingCodecSpec {
	cloned := cloneDaemonHookDecl(decl)
	return hookBindingCodecSpec{
		Name:         cloned.Name,
		Event:        cloned.Event,
		Source:       cloned.Source,
		Mode:         cloned.Mode,
		Required:     cloned.Required,
		Priority:     cloned.Priority,
		PrioritySet:  cloned.PrioritySet,
		Timeout:      cloned.Timeout,
		Matcher:      cloned.Matcher,
		ExecutorKind: cloned.ExecutorKind,
		Command:      cloned.Command,
		Args:         cloned.Args,
		WorkingDir:   cloned.WorkingDir,
		Env:          cloned.Env,
		SecretEnv:    cloned.SecretEnv,
		Metadata:     cloned.Metadata,
		SkillSource:  cloned.SkillSource,
	}
}

func (s *hookBindingCodecSpec) hookDecl() hookspkg.HookDecl {
	return cloneDaemonHookDecl(hookspkg.HookDecl{
		Name:         s.Name,
		Event:        s.Event,
		Source:       s.Source,
		Mode:         s.Mode,
		Required:     s.Required,
		Priority:     s.Priority,
		PrioritySet:  s.PrioritySet,
		Timeout:      s.Timeout,
		Matcher:      s.Matcher,
		ExecutorKind: s.ExecutorKind,
		Command:      s.Command,
		Args:         s.Args,
		WorkingDir:   s.WorkingDir,
		Env:          s.Env,
		SecretEnv:    s.SecretEnv,
		Metadata:     s.Metadata,
		SkillSource:  s.SkillSource,
	})
}

func newHookBindingStore(
	raw resources.RawStore,
	codec resources.KindCodec[hookspkg.HookDecl],
) (resources.Store[hookspkg.HookDecl], error) {
	return resources.NewStore(raw, codec)
}

func newHookBindingProjector(runtime *hookspkg.Hooks) resources.TypedProjector[hookspkg.HookDecl] {
	if runtime == nil {
		return nil
	}
	return &hookBindingProjector{runtime: runtime}
}

func validateHookBindingSpec(
	_ context.Context,
	scope resources.ResourceScope,
	spec hookspkg.HookDecl,
) (hookspkg.HookDecl, error) {
	normalizedScope := scope.Normalize()
	if err := normalizedScope.Validate("scope"); err != nil {
		return hookspkg.HookDecl{}, err
	}

	normalized, err := hookspkg.CanonicalizeHookDecl(spec)
	if err != nil {
		return hookspkg.HookDecl{}, err
	}

	if normalizedScope.Kind == resources.ResourceScopeKindWorkspace {
		workspaceID := strings.TrimSpace(normalized.Matcher.WorkspaceID)
		switch {
		case workspaceID == "":
			normalized.Matcher.WorkspaceID = normalizedScope.ID
		case workspaceID != normalizedScope.ID:
			return hookspkg.HookDecl{}, fmt.Errorf(
				"%w: hook workspace matcher %q does not match resource scope %q",
				resources.ErrInvalidScopeBinding,
				workspaceID,
				normalizedScope.ID,
			)
		}
	}

	return normalized, nil
}

func (p *hookBindingProjector) Kind() resources.ResourceKind {
	return hookBindingResourceKind
}

func (p *hookBindingProjector) DependsOn() []resources.ResourceKind {
	return nil
}

func (p *hookBindingProjector) Build(
	_ context.Context,
	records []resources.Record[hookspkg.HookDecl],
) (resources.ProjectionPlan, error) {
	if p == nil || p.runtime == nil {
		return nil, errors.New("daemon: hook binding projector runtime is required")
	}

	decls := make([]hookspkg.HookDecl, 0, len(records))
	var revision int64
	for _, record := range records {
		if record.Version > revision {
			revision = record.Version
		}
		decls = append(decls, cloneDaemonHookDecl(record.Spec))
	}

	state, err := p.runtime.BuildBindingState(decls)
	if err != nil {
		return nil, err
	}

	return &hookBindingProjectionPlan{
		revision:   revision,
		operations: state.HookCount(),
		state:      state,
	}, nil
}

func (p *hookBindingProjector) Apply(ctx context.Context, plan resources.ProjectionPlan) error {
	if p == nil || p.runtime == nil {
		return errors.New("daemon: hook binding projector runtime is required")
	}
	if ctx == nil {
		return errors.New("daemon: hook binding projector apply context is required")
	}

	typed, ok := plan.(*hookBindingProjectionPlan)
	if !ok {
		return fmt.Errorf("daemon: hook binding projector plan has type %T", plan)
	}

	return p.runtime.ApplyBindingState(typed.state, typed.revision)
}
