package state

import (
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
