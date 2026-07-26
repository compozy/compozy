package skills

import (
	"context"
	"errors"

	"maps"

	"strings"

	"github.com/compozy/agh/internal/filesnap"
)

func (r *Registry) loadGlobalSkills(
	ctx context.Context,
	disabledSkills []string,
) (map[string]*Skill, map[string]filesnap.Snapshot, []SkillDiagnostic, error) {
	skills := make(map[string]*Skill)
	snapshots := make(map[string]filesnap.Snapshot)
	diagnostics := make([]SkillDiagnostic, 0)

	if err := r.loadBundledSkills(ctx, skills, disabledSkills, &diagnostics); err != nil {
		return nil, nil, nil, err
	}
	if err := r.loadDirectorySkills(
		ctx,
		r.cfg.UserSkillsDir,
		SourceUser,
		skills,
		snapshots,
		disabledSkills,
		&diagnostics,
	); err != nil {
		return nil, nil, nil, err
	}

	return skills, snapshots, diagnostics, nil
}

func (r *Registry) loadWorkspaceSkills(
	ctx context.Context,
	paths []workspaceSkillPath,
	disabledSkills []string,
) (map[string]*Skill, []SkillDiagnostic, error) {
	skills := make(map[string]*Skill)
	diagnostics := make([]SkillDiagnostic, 0)

	for _, path := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return nil, nil, err
		}

		skill, content, err := parseSkillFileDocument(path.filePath)
		if err != nil {
			return nil, nil, err
		}
		skill.Source = path.source
		refreshSkillHookDecls(skill)
		if !r.processSkillWithDiagnostics(skills, skill, content, disabledSkills, &diagnostics) {
			continue
		}
	}

	return skills, diagnostics, nil
}

func (r *Registry) loadBundledSkills(
	ctx context.Context,
	dst map[string]*Skill,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	if r.cfg.BundledFS == nil {
		return nil
	}

	paths, err := scanBundledFS(r.cfg.BundledFS)
	if err != nil {
		return err
	}

	for _, skillPath := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return err
		}

		skill, content, err := parseBundledSkillDocument(r.cfg.BundledFS, skillPath)
		if err != nil {
			return err
		}
		if !r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, diagnostics) {
			continue
		}
	}

	return nil
}

func (r *Registry) loadDirectorySkills(
	ctx context.Context,
	dir string,
	source SkillSource,
	dst map[string]*Skill,
	snapshots map[string]filesnap.Snapshot,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	root := strings.TrimSpace(dir)
	if root == "" {
		return nil
	}

	paths, dirSnapshots, err := scanDirectoryWithSnapshots(root)
	if err != nil {
		return err
	}
	maps.Copy(snapshots, dirSnapshots)
	if err := recordSidecarSnapshots(paths, snapshots); err != nil {
		return err
	}

	return r.loadSkillPaths(ctx, paths, source, dst, disabledSkills, diagnostics)
}

func (r *Registry) loadSkillPaths(
	ctx context.Context,
	paths []string,
	source SkillSource,
	dst map[string]*Skill,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) error {
	for _, skillPath := range paths {
		if err := checkRegistryContext(ctx); err != nil {
			return err
		}

		skill, content, err := parseSkillFileDocument(skillPath)
		if err != nil {
			return err
		}
		if err := r.assignSourceAndProvenance(skill, source); err != nil {
			return err
		}
		if !r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, diagnostics) {
			continue
		}
	}

	return nil
}

func (r *Registry) processSkill(dst map[string]*Skill, skill *Skill, content string, disabledSkills []string) bool {
	return r.processSkillWithDiagnostics(dst, skill, content, disabledSkills, nil)
}

func (r *Registry) processSkillWithDiagnostics(
	dst map[string]*Skill,
	skill *Skill,
	content string,
	disabledSkills []string,
	diagnostics *[]SkillDiagnostic,
) bool {
	r.applyDisabled(skill, disabledSkills)

	verifyErr := r.verifyMarketplaceSkill(skill)
	warnings := VerifyContent(content)
	r.logVerificationWarnings(skill, warnings)
	if verifyErr != nil {
		appendSkillDiagnostic(diagnostics, skillVerificationFailedDiagnostic(skill, verifyErr, warnings))
		return false
	}
	if hasCriticalWarning(warnings) {
		appendSkillDiagnostic(diagnostics, skillVerificationFailedDiagnostic(skill, nil, warnings))
		return false
	}

	skill.Diagnostics.VerificationStatus = verificationStatusForWarnings(warnings)
	skill.Diagnostics.Warnings = cloneWarnings(warnings)
	r.overlaySkill(dst, skill)
	return true
}

func (r *Registry) assignSourceAndProvenance(skill *Skill, source SkillSource) error {
	if skill == nil {
		return errors.New("skills: skill is required")
	}

	skill.Source = source
	if source != SourceUser {
		refreshSkillHookDecls(skill)
		return nil
	}

	hasSidecar, err := HasSidecar(skill.Dir)
	if err != nil {
		return err
	}
	if !hasSidecar {
		refreshSkillHookDecls(skill)
		return nil
	}

	provenance, err := ReadSidecar(skill.Dir)
	if err != nil {
		return err
	}

	skill.Source = SourceMarketplace
	skill.Provenance = provenance
	skill.InstalledFrom = strings.TrimSpace(provenance.Slug)
	refreshSkillHookDecls(skill)

	return nil
}

func (r *Registry) verifyMarketplaceSkill(skill *Skill) error {
	if skill == nil || skill.Source != SourceMarketplace || skill.Provenance == nil {
		return nil
	}

	err := VerifyHash(skill.Dir, skill.Provenance)
	if err == nil {
		return nil
	}

	if mismatch, ok := errors.AsType[*HashMismatchError](err); ok {
		r.logger.Warn(
			"skills: marketplace skill hash mismatch",
			"skill_name", skill.Meta.Name,
			"expected_hash", mismatch.ExpectedHash,
			"actual_hash", mismatch.ActualHash,
			"path", skill.FilePath,
		)
		return err
	}

	r.logger.Warn(
		"skills: marketplace skill hash verification failed",
		"skill_name", skill.Meta.Name,
		"path", skill.FilePath,
		"error", err,
	)

	return err
}
