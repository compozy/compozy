package state

import (
	"fmt"

	"github.com/compozy/compozy/engine/common"
	"github.com/compozy/compozy/pkg/tplengine"
)

// -----------------------------------------------------------------------------
// Initializer Interface
// -----------------------------------------------------------------------------

type Initializer interface {
	Initialize() (*State, error)
	MergeEnv(parentEnv common.EnvMap, componentEnv common.EnvMap) (*common.EnvMap, error)
}

// -----------------------------------------------------------------------------
// Common Initializer Implementation
// -----------------------------------------------------------------------------

type CommonInitializer struct {
	Normalizer Normalizer
}

func NewCommonInitializer() *CommonInitializer {
	return &CommonInitializer{
		Normalizer: *NewNormalizer(tplengine.FormatYAML),
	}
}

func (ci *CommonInitializer) MergeEnv(parentEnv common.EnvMap, componentEnv common.EnvMap) (*common.EnvMap, error) {
	result := make(common.EnvMap)
	result, err := result.Merge(parentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge parent env: %w", err)
	}
	result, err = result.Merge(componentEnv)
	if err != nil {
		return nil, fmt.Errorf("failed to merge component env: %w", err)
	}
	return &result, nil
}
