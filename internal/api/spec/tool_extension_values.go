package spec

import (
	"sort"

	extensioncontract "github.com/compozy/agh/internal/extension/contract"

	"github.com/compozy/agh/internal/tools"
)

func toolSourceValues() []string {
	return []string{"builtin", specMCPKey, specExtensionKey, "dynamic"}
}

func toolBackendKindValues() []string {
	return []string{
		string(tools.BackendNativeGo),
		string(tools.BackendExtensionHost),
		string(tools.BackendMCP),
		string(tools.BackendBridge),
	}
}

func toolVisibilityValues() []string {
	return []string{
		string(tools.VisibilityInternal),
		string(tools.VisibilityOperator),
		string(tools.VisibilitySession),
		string(tools.VisibilityModel),
	}
}

func toolRiskClassValues() []string {
	return []string{
		string(tools.RiskRead),
		string(tools.RiskMutating),
		string(tools.RiskOpenWorld),
		string(tools.RiskDestructive),
	}
}

func toolCallEventKindValues() []string {
	return []string{
		string(tools.ToolCallStarted),
		string(tools.ToolCallCompleted),
		string(tools.ToolCallFailed),
		string(tools.ToolCallDenied),
		string(tools.ToolResultTruncated),
	}
}

func hostAPIMethodValues() []string {
	specs := extensioncontract.HostAPIMethodSpecs()
	values := make([]string, 0, len(specs))
	for _, spec := range specs {
		values = append(values, string(spec.Method))
	}
	sort.Strings(values)
	return values
}
