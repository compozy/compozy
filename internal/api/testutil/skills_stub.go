package testutil

import (
	"context"

	core "github.com/compozy/agh/internal/api/core"
	"github.com/compozy/agh/internal/skills"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

type StubSkillsRegistry struct {
	GetFn          func(name string) (*skills.Skill, bool)
	ListFn         func() []*skills.Skill
	ForWorkspaceFn func(ctx context.Context, resolved *workspacepkg.ResolvedWorkspace) ([]*skills.Skill, error)
	ForAgentFn     func(
		ctx context.Context,
		resolved *workspacepkg.ResolvedWorkspace,
		agentName string,
	) ([]*skills.Skill, error)
	LoadContentFn        func(ctx context.Context, skill *skills.Skill) (string, error)
	LoadResourceFn       func(ctx context.Context, skill *skills.Skill, relativePath string) (string, error)
	SetEnabledFn         func(name string, resolved *workspacepkg.ResolvedWorkspace, enabled bool) error
	SetEnabledForAgentFn func(name string, resolved *workspacepkg.ResolvedWorkspace, agentName string, enabled bool) error
	RefreshGlobalFn      func(ctx context.Context) error
}

func (s StubSkillsRegistry) Get(name string) (*skills.Skill, bool) {
	if s.GetFn != nil {
		return s.GetFn(name)
	}
	return nil, false
}

func (s StubSkillsRegistry) List() []*skills.Skill {
	if s.ListFn != nil {
		return s.ListFn()
	}
	return nil
}

func (s StubSkillsRegistry) ForWorkspace(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]*skills.Skill, error) {
	if s.ForWorkspaceFn != nil {
		return s.ForWorkspaceFn(ctx, resolved)
	}
	return nil, nil
}

func (s StubSkillsRegistry) ForAgent(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]*skills.Skill, error) {
	if s.ForAgentFn != nil {
		return s.ForAgentFn(ctx, resolved, agentName)
	}
	if s.ForWorkspaceFn != nil {
		return s.ForWorkspaceFn(ctx, resolved)
	}
	return nil, nil
}

func (s StubSkillsRegistry) LoadContent(ctx context.Context, skill *skills.Skill) (string, error) {
	if s.LoadContentFn != nil {
		return s.LoadContentFn(ctx, skill)
	}
	return "", nil
}

func (s StubSkillsRegistry) LoadResource(
	ctx context.Context,
	skill *skills.Skill,
	relativePath string,
) (string, error) {
	if s.LoadResourceFn != nil {
		return s.LoadResourceFn(ctx, skill, relativePath)
	}
	return "", nil
}

func (s StubSkillsRegistry) SetEnabled(name string, resolved *workspacepkg.ResolvedWorkspace, enabled bool) error {
	if s.SetEnabledFn != nil {
		return s.SetEnabledFn(name, resolved, enabled)
	}
	return nil
}

func (s StubSkillsRegistry) SetEnabledForAgent(
	name string,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	enabled bool,
) error {
	if s.SetEnabledForAgentFn != nil {
		return s.SetEnabledForAgentFn(name, resolved, agentName, enabled)
	}
	if s.SetEnabledFn != nil {
		return s.SetEnabledFn(name, resolved, enabled)
	}
	return nil
}

func (s StubSkillsRegistry) RefreshGlobal(ctx context.Context) error {
	if s.RefreshGlobalFn != nil {
		return s.RefreshGlobalFn(ctx)
	}
	return nil
}

var _ core.SkillsRegistry = (*StubSkillsRegistry)(nil)
var _ core.SkillsRegistryRefresher = (*StubSkillsRegistry)(nil)
