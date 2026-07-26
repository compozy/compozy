package skills

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	aghconfig "github.com/compozy/agh/internal/config"
	workspacepkg "github.com/compozy/agh/internal/workspace"
)

var (
	// ErrAgentNotFound reports that the requested agent is unavailable in the selected scope.
	ErrAgentNotFound = errors.New("skills: agent not found")
	// ErrAgentLocalInvalid reports an invalid AGENT.md or agent-local skills layer.
	ErrAgentLocalInvalid = errors.New("skills: invalid agent-local layer")

	errAgentLocalVerification = errors.New("skills: agent-local verification failed")
)

// ForAgent returns the effective skill set for one logical agent after applying
// the final agent-local overlay over the current global/workspace-effective set.
func (r *Registry) ForAgent(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) ([]*Skill, error) {
	return r.ForAgentSession(ctx, resolved, agentName, "")
}

// ForAgentSession returns the effective offer-time skill projection for one session.
func (r *Registry) ForAgentSession(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	sessionID string,
) ([]*Skill, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}

	target := aghconfig.NormalizeAgentName(agentName)
	if err := aghconfig.ValidateAgentName(target); err != nil {
		return nil, err
	}

	agent, err := r.resolveAgentScope(resolved, target)
	if err != nil {
		if errors.Is(err, ErrAgentLocalInvalid) {
			r.emitSkillsLoadFailed(ctx, resourceWorkspaceKey(resolved), target, err)
		}
		return nil, err
	}
	return r.ForAgentDefSession(ctx, resolved, agent, sessionID)
}

// ForAgentDef returns the effective skill set for an already-resolved concrete
// agent definition.
func (r *Registry) ForAgentDef(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agent aghconfig.AgentDef,
) ([]*Skill, error) {
	return r.ForAgentDefSession(ctx, resolved, agent, "")
}

// ForAgentDefSession projects offer-time skill activation from the concrete
// definition that owns the session, including extension and bundle agents that
// are not authored on disk.
func (r *Registry) ForAgentDefSession(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agent aghconfig.AgentDef,
	sessionID string,
) ([]*Skill, error) {
	if err := checkRegistryContext(ctx); err != nil {
		return nil, err
	}

	target := aghconfig.NormalizeAgentName(agent.Name)
	if err := aghconfig.ValidateAgentName(target); err != nil {
		return nil, err
	}
	agent = aghconfig.CloneAgentDef(agent)
	agent.Name = target

	baseSkills, err := r.baseSkillsForAgent(ctx, resolved)
	if err != nil {
		return nil, err
	}
	skillsByName := cloneSkillMapFromList(baseSkills)
	if strings.TrimSpace(agent.SourcePath) == "" {
		applyForcedDisabledSkills(skillsByName, agent.Skills.Disabled)
		return r.projectAgentSkillActivation(ctx, resolved, agent, sessionID, mergedSkillList(nil, skillsByName))
	}

	agentSkillsDir := filepath.Join(filepath.Dir(agent.SourcePath), aghconfig.SkillsDirName)
	agentLocalSkills, err := r.loadAgentLocalSkills(ctx, agentSkillsDir, target, agent.Skills.Disabled)
	if err != nil {
		r.emitSkillsLoadFailed(ctx, resourceWorkspaceKey(resolved), target, err)
		return nil, err
	}
	r.emitEventSummaries(
		ctx,
		r.buildSkillShadowSummaries(
			skillsByName,
			agentLocalSkills,
			"agent",
			resourceWorkspaceKey(resolved),
			target,
		),
	)
	for _, skill := range agentLocalSkills {
		r.overlaySkill(skillsByName, skill)
	}
	applyForcedDisabledSkills(skillsByName, agent.Skills.Disabled)

	return r.projectAgentSkillActivation(ctx, resolved, agent, sessionID, mergedSkillList(nil, skillsByName))
}

func (r *Registry) projectAgentSkillActivation(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
	agent aghconfig.AgentDef,
	sessionID string,
	skills []*Skill,
) ([]*Skill, error) {
	capabilities := make([]string, 0)
	if agent.Capabilities != nil {
		capabilities = make([]string, 0, len(agent.Capabilities.Capabilities))
		for _, capability := range agent.Capabilities.Capabilities {
			if id := strings.TrimSpace(capability.ID); id != "" {
				capabilities = append(capabilities, id)
			}
		}
	}
	return r.projectSkillActivation(ctx, skills, ActivationTarget{
		WorkspaceID:  resourceWorkspaceKey(resolved),
		SessionID:    strings.TrimSpace(sessionID),
		AgentName:    strings.TrimSpace(agent.Name),
		Capabilities: capabilities,
	})
}

// SetEnabledForAgent persists an agent-scoped logical tombstone in the winning AGENT.md.
func (r *Registry) SetEnabledForAgent(
	name string,
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
	enabled bool,
) error {
	targetSkill := strings.TrimSpace(name)
	if targetSkill == "" {
		return errors.New("skills: skill name is required")
	}

	targetAgent := aghconfig.NormalizeAgentName(agentName)
	if err := aghconfig.ValidateAgentName(targetAgent); err != nil {
		return err
	}

	agent, err := r.resolveAgentScope(resolved, targetAgent)
	if err != nil {
		return err
	}
	if strings.TrimSpace(agent.SourcePath) == "" {
		return fmt.Errorf("%w: agent %q source path is required", ErrAgentLocalInvalid, targetAgent)
	}

	_, err = aghconfig.EditAgentDefFile(agent.SourcePath, func(def *aghconfig.AgentDef) error {
		if def == nil {
			return errors.New("skills: agent definition is required")
		}
		def.Skills.Disabled = setDisabledSkill(def.Skills.Disabled, targetSkill, enabled)
		return nil
	})
	if err != nil {
		return fmt.Errorf("skills: edit agent %q: %w", targetAgent, err)
	}
	r.globalVersion.Add(1)
	return nil
}

func (r *Registry) baseSkillsForAgent(
	ctx context.Context,
	resolved *workspacepkg.ResolvedWorkspace,
) ([]*Skill, error) {
	if resolved == nil {
		return r.rawList(), nil
	}
	return r.workspaceSkills(ctx, resolved)
}

func (r *Registry) resolveAgentScope(
	resolved *workspacepkg.ResolvedWorkspace,
	agentName string,
) (aghconfig.AgentDef, error) {
	if resolved != nil {
		for _, diagnostic := range resolved.AgentDiagnostics {
			if aghconfig.NormalizeAgentName(diagnostic.Name) != agentName {
				continue
			}
			return aghconfig.AgentDef{}, fmt.Errorf(
				"%w: agent %q at %q: %s",
				ErrAgentLocalInvalid,
				agentName,
				strings.TrimSpace(diagnostic.Path),
				strings.TrimSpace(diagnostic.Message),
			)
		}
		for _, agent := range resolved.Agents {
			if aghconfig.NormalizeAgentName(agent.Name) != agentName {
				continue
			}
			return aghconfig.CloneAgentDef(agent), nil
		}
		if builtin, ok := aghconfig.BuiltinAgentDef(agentName); ok {
			return builtin, nil
		}
		return aghconfig.AgentDef{}, fmt.Errorf("%w: %q", ErrAgentNotFound, agentName)
	}

	agentsDir := r.globalAgentsDir()
	if strings.TrimSpace(agentsDir) == "" {
		if builtin, ok := aghconfig.BuiltinAgentDef(agentName); ok {
			return builtin, nil
		}
		return aghconfig.AgentDef{}, fmt.Errorf("%w: %q", ErrAgentNotFound, agentName)
	}
	path := filepath.Join(agentsDir, agentName, "AGENT.md")
	agent, err := aghconfig.LoadAgentDefFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if builtin, ok := aghconfig.BuiltinAgentDef(agentName); ok {
				return builtin, nil
			}
			return aghconfig.AgentDef{}, fmt.Errorf("%w: %q", ErrAgentNotFound, agentName)
		}
		return aghconfig.AgentDef{}, fmt.Errorf(
			"%w",
			newAgentLocalLoadError(
				"validation",
				path,
				fmt.Sprintf("agent %q: %v", agentName, err),
				ErrAgentLocalInvalid,
			),
		)
	}
	if aghconfig.NormalizeAgentName(agent.Name) != agentName {
		return aghconfig.AgentDef{}, fmt.Errorf("%w", newAgentLocalLoadError(
			"validation",
			path,
			fmt.Sprintf("agent file %q defines %q, expected %q", path, agent.Name, agentName),
			ErrAgentLocalInvalid,
		))
	}
	return agent, nil
}

func (r *Registry) loadAgentLocalSkills(
	ctx context.Context,
	root string,
	agentName string,
	disabledSkills []string,
) (map[string]*Skill, error) {
	trimmedRoot := strings.TrimSpace(root)
	if trimmedRoot == "" {
		return nil, nil
	}

	paths, _, err := scanDirectoryWithSnapshots(trimmedRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, wrapAgentLocalLoadError("sidecar", trimmedRoot, agentName, err)
	}
	if len(paths) == 0 {
		return nil, nil
	}

	skills := make(map[string]*Skill, len(paths))
	for _, skillPath := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return nil, err
		}
		if err := r.loadAgentLocalSkill(skills, skillPath, agentName, disabledSkills); err != nil {
			return nil, err
		}
	}

	return skills, nil
}

func (r *Registry) loadAgentLocalSkill(
	dst map[string]*Skill,
	skillPath string,
	agentName string,
	disabledSkills []string,
) error {
	skill, content, parseErr := parseSkillFileDocument(skillPath)
	if parseErr != nil {
		return wrapAgentLocalLoadError("parse", skillPath, agentName, parseErr)
	}
	if err := r.assignSourceAndProvenance(skill, SourceAgentLocal); err != nil {
		return wrapAgentLocalLoadError("validation", skillPath, agentName, err)
	}
	if err := r.processSkillStrict(dst, skill, content, disabledSkills); err != nil {
		code := "validation"
		if errors.Is(err, errAgentLocalVerification) {
			code = "verification"
		}
		return wrapAgentLocalLoadError(code, skillPath, agentName, err)
	}
	return nil
}

func wrapAgentLocalLoadError(code string, path string, agentName string, err error) error {
	detail := fmt.Sprintf("agent %q skill %q: %v", agentName, path, err)
	if code == "sidecar" {
		detail = fmt.Sprintf("agent %q skills root %q: %v", agentName, path, err)
	}
	return fmt.Errorf(
		"%w",
		newAgentLocalLoadError(code, path, detail, ErrAgentLocalInvalid),
	)
}

func (r *Registry) processSkillStrict(
	dst map[string]*Skill,
	skill *Skill,
	content string,
	disabledSkills []string,
) error {
	r.applyDisabled(skill, disabledSkills)

	verifyErr := r.verifyMarketplaceSkill(skill)
	warnings := VerifyContent(content)
	r.logVerificationWarnings(skill, warnings)
	if verifyErr != nil {
		return fmt.Errorf("%w: %w", errAgentLocalVerification, verifyErr)
	}
	for _, warning := range warnings {
		if warning.Severity != SeverityCritical {
			continue
		}
		return fmt.Errorf("%w: %s", errAgentLocalVerification, strings.TrimSpace(warning.Message))
	}

	skill.Diagnostics.VerificationStatus = verificationStatusForWarnings(warnings)
	skill.Diagnostics.Warnings = cloneWarnings(warnings)
	r.overlaySkill(dst, skill)
	return nil
}

func applyForcedDisabledSkills(skillsByName map[string]*Skill, disabledSkills []string) {
	if len(disabledSkills) == 0 || len(skillsByName) == 0 {
		return
	}
	for _, name := range disabledSkills {
		skill := skillsByName[strings.TrimSpace(name)]
		if skill == nil {
			continue
		}
		skill.Enabled = false
	}
}

func cloneSkillMapFromList(skills []*Skill) map[string]*Skill {
	if len(skills) == 0 {
		return map[string]*Skill{}
	}
	cloned := make(map[string]*Skill, len(skills))
	for _, skill := range skills {
		if skill == nil {
			continue
		}
		cloned[strings.TrimSpace(skill.Meta.Name)] = cloneSkill(skill)
	}
	return cloned
}

func (r *Registry) globalAgentsDir() string {
	if userAgentsDir := strings.TrimSpace(r.cfg.UserAgentsDir); userAgentsDir != "" {
		return userAgentsDir
	}
	userSkillsDir := strings.TrimSpace(r.cfg.UserSkillsDir)
	if userSkillsDir == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(userSkillsDir), aghconfig.AgentsDirName)
}
