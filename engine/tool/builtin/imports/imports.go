package imports

import (
	// Register cp__ native builtins with the shared registry during initialization.
	_ "github.com/compozy/compozy/engine/tool/builtin/agentcatalog"
	_ "github.com/compozy/compozy/engine/tool/builtin/callagent"
	_ "github.com/compozy/compozy/engine/tool/builtin/callagents"
	_ "github.com/compozy/compozy/engine/tool/builtin/calltask"
	_ "github.com/compozy/compozy/engine/tool/builtin/calltasks"
	_ "github.com/compozy/compozy/engine/tool/builtin/callworkflow"
	_ "github.com/compozy/compozy/engine/tool/builtin/callworkflows"
)
