package loop

import (
	"context"
	"fmt"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/resources"
)

func validateResourceSpec(
	_ context.Context,
	scope resources.ResourceScope,
	spec ResourceSpec,
) (ResourceSpec, error) {
	normalized := CloneResourceSpec(spec)
	normalized.Name = strings.TrimSpace(normalized.Name)
	normalized.Description = strings.TrimSpace(normalized.Description)
	normalized.Source = normalized.Source.Normalize()
	normalized.Dir = strings.TrimSpace(normalized.Dir)
	normalized.FilePath = strings.TrimSpace(normalized.FilePath)
	normalized.DefinitionChecksum = strings.TrimSpace(normalized.DefinitionChecksum)
	normalized.InstalledFromExtension = strings.TrimSpace(normalized.InstalledFromExtension)
	normalized.Catalog = normalizeCatalogSpec(normalized.Catalog)
	normalized.Start = normalizeStartSpecs(normalized.Start)

	if err := scope.Normalize().Validate("scope"); err != nil {
		return ResourceSpec{}, err
	}
	if normalized.Name == "" {
		return ResourceSpec{}, fmt.Errorf("%w: loop.name is required", resources.ErrValidation)
	}
	if _, err := ValidateName(normalized.Name); err != nil {
		return ResourceSpec{}, fmt.Errorf("%w: %v", resources.ErrValidation, err)
	}
	if !normalized.Source.Valid() {
		return ResourceSpec{}, fmt.Errorf("%w: loop.source is invalid: %q", resources.ErrValidation, normalized.Source)
	}
	if normalized.FilePath == "" {
		return ResourceSpec{}, fmt.Errorf("%w: file-backed loops require file_path", resources.ErrValidation)
	}
	for _, start := range normalized.Start {
		if !knownStartKind(start.Kind) {
			return ResourceSpec{}, fmt.Errorf("%w: loop.start kind is invalid: %q", resources.ErrValidation, start.Kind)
		}
	}
	return normalized, nil
}

func validateDefinitionForProjection(def dsl.Definition, linter Linter) error {
	if strings.TrimSpace(def.Meta.Name) == "" {
		return fmt.Errorf("%w: loop meta.name is required", resources.ErrValidation)
	}
	if _, err := ValidateName(def.Meta.Name); err != nil {
		return fmt.Errorf("%w: %v", resources.ErrValidation, err)
	}
	for _, binding := range def.Start {
		if !knownStartKind(string(binding.Kind)) {
			return fmt.Errorf("%w: loop start.kind is invalid: %q", resources.ErrValidation, binding.Kind)
		}
	}
	if linter == nil {
		return nil
	}
	for _, diagnostic := range linter.Lint(def) {
		if diagnostic.Severity == SeverityError {
			return fmt.Errorf(
				"%w: loop definition lint error %s: %s",
				resources.ErrValidation,
				diagnostic.Code,
				diagnostic.Message,
			)
		}
	}
	return nil
}

func knownStartKind(kind string) bool {
	switch dsl.StartKind(strings.TrimSpace(kind)) {
	case dsl.StartManual, dsl.StartCLI, dsl.StartHTTP, dsl.StartUDS, dsl.StartTrigger,
		dsl.StartSchedule, dsl.StartWebhook, dsl.StartNetwork, dsl.StartExtension, dsl.StartNativeTool:
		return true
	default:
		return false
	}
}
