package project

import "github.com/compozy/compozy/engine/core"

// Opts contains project-specific configuration options.
//
// **Features**:
// - **Project-level settings**: Configuration specific to the project
// - **Activity options**: Shared across all Compozy components
//
// **Note**: System-level limits and dispatcher settings are configured server-side
// in the pkg/config package and are not exposed in project configuration.
type Opts struct {
	// GlobalOpts embeds common configuration options shared across all Compozy components
	core.GlobalOpts `json:",inline" yaml:",inline" mapstructure:",squash"`
}
