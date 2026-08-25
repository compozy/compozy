package skills

import (
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/resources"
)

func testGlobalSkillRoots(dir string) []compozyconfig.SkillRootSpec {
	return []compozyconfig.SkillRootSpec{{
		Dir:           dir,
		SourceSlug:    compozyconfig.SkillSourceCompozy,
		Kind:          compozyconfig.RootKindBuiltin,
		ResourceScope: resources.ResourceScope{Kind: resources.ResourceScopeKindUser},
	}}
}
