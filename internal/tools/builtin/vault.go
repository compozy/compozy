package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var vaultTools = []toolspkg.Descriptor{
	nativeDescriptor(
		toolspkg.ToolIDVaultList,
		"vault_list",
		"Vault List",
		"List global Vault secret metadata without exposing secret values.",
		vaultListInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		false,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDCatalog},
		[]string{"vault", "secrets", descriptorKeywordCatalog, "metadata"},
		[]string{"vault refs", "secret references", "list secret metadata"},
	),
}

func vaultDescriptors() []toolspkg.Descriptor {
	return vaultTools
}

const vaultListInputSchema = `{
	"type":"object",
	"properties":{"prefix":{"type":"string"}},
	"additionalProperties":false
}`
