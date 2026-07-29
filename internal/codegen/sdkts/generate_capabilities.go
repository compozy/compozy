package sdkts

import (
	"strconv"
	"strings"

	extensioncontract "github.com/compozy/compozy/internal/extension/contract"
)

func (g *generator) emitCapabilityContracts() {
	contracts := extensioncontract.ProvideCapabilityContracts()

	var required strings.Builder
	required.WriteString("export const REQUIRED_METHODS_BY_PROVIDE = {\n")
	for _, contract := range contracts {
		required.WriteString("  ")
		required.WriteString(strconv.Quote(contract.Provide))
		required.WriteString(": [")
		for index, method := range contract.Methods {
			if index > 0 {
				required.WriteString(", ")
			}
			required.WriteString(strconv.Quote(method))
		}
		required.WriteString("],\n")
	}
	required.WriteString("} as const satisfies Readonly<Record<string, readonly string[]>>;\n")
	g.blocks = append(g.blocks, required.String())

	var fixtures strings.Builder
	fixtures.WriteString("export interface ProvideConformanceFixture {\n")
	fixtures.WriteString("  provide: string;\n")
	fixtures.WriteString("  required_methods: readonly string[];\n")
	fixtures.WriteString("}\n\n")
	fixtures.WriteString("export const PUBLIC_PROVIDE_CONFORMANCE_FIXTURES: readonly ProvideConformanceFixture[] = [\n")
	for _, contract := range contracts {
		if !contract.Public {
			continue
		}
		fixtures.WriteString("  { provide: ")
		fixtures.WriteString(strconv.Quote(contract.Provide))
		fixtures.WriteString(", required_methods: REQUIRED_METHODS_BY_PROVIDE[")
		fixtures.WriteString(strconv.Quote(contract.Provide))
		fixtures.WriteString("] },\n")
	}
	fixtures.WriteString("];\n")
	g.blocks = append(g.blocks, fixtures.String())
}
