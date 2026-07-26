package loop

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/compozy/agh/internal/loop/dsl"
	"github.com/compozy/agh/internal/resources"
)

// ParseResource projects one YAML loop definition into a metadata resource spec.
func ParseResource(data []byte, opts ResourceParseOptions) (ResourceSpec, dsl.Definition, error) {
	spec, def, err := projectResource(data, opts)
	if err != nil {
		return ResourceSpec{}, dsl.Definition{}, err
	}
	if _, err := validateResourceSpec(
		context.Background(),
		resources.ResourceScope{Kind: resources.ResourceScopeKindGlobal},
		spec,
	); err != nil {
		return ResourceSpec{}, dsl.Definition{}, err
	}
	return spec, def, nil
}

func projectResource(data []byte, opts ResourceParseOptions) (ResourceSpec, dsl.Definition, error) {
	def, err := dsl.Parse(data)
	if err != nil {
		return ResourceSpec{}, dsl.Definition{}, fmt.Errorf("loop: parse definition: %w", err)
	}
	if err := validateDefinitionForProjection(def, opts.Linter); err != nil {
		return ResourceSpec{}, dsl.Definition{}, err
	}
	spec := ResourceSpecFromDefinition(def, opts)
	spec.DefinitionChecksum = checksumBytes(data)
	return spec, def, nil
}

// ParseResourceFile reads and projects one YAML loop definition from disk.
func ParseResourceFile(path string, opts ResourceParseOptions) (ResourceSpec, dsl.Definition, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ResourceSpec{}, dsl.Definition{}, fmt.Errorf("loop: read definition %q: %w", path, err)
	}
	opts.FilePath = path
	if opts.Dir == "" {
		opts.Dir = filepath.Dir(path)
	}
	return ParseResource(data, opts)
}

// ResourceSpecFromDefinition extracts metadata from a parsed loop definition.
func ResourceSpecFromDefinition(def dsl.Definition, opts ResourceParseOptions) ResourceSpec {
	return ResourceSpec{
		Name:                   strings.TrimSpace(def.Meta.Name),
		Version:                def.Meta.Version,
		Description:            strings.TrimSpace(def.Meta.Description),
		Catalog:                catalogSpecFromDefinition(def),
		ContractGoal:           strings.TrimSpace(def.Contract.Goal),
		Start:                  startSpecsFromDefinition(def),
		Source:                 opts.Source.Normalize(),
		Dir:                    strings.TrimSpace(opts.Dir),
		FilePath:               strings.TrimSpace(opts.FilePath),
		InstalledFromExtension: strings.TrimSpace(opts.InstalledFromExtension),
	}
}

// CloneResourceSpec returns a deep copy of a projected loop resource.
func CloneResourceSpec(spec ResourceSpec) ResourceSpec {
	spec.Catalog.Keywords = append([]string(nil), spec.Catalog.Keywords...)
	spec.Start = cloneStartSpecs(spec.Start)
	return spec
}

func checksumBytes(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
