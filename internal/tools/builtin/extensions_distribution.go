package builtin

import toolspkg "github.com/compozy/compozy/internal/tools"

var extensionDistributionTools = []toolspkg.Descriptor{
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsSearch,
		"extensions_search",
		"Extensions Search",
		"Discover extensions across curated and GitHub sources.",
		extensionSearchInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsCatalogKey, descriptorKeywordSearch},
		[]string{"search extensions", "discover extension"},
	),
	nativeExtensionDescriptor(
		toolspkg.ToolIDExtensionsProvenance,
		"extensions_provenance",
		"Extensions Provenance",
		"Read one installed extension's source and integrity provenance.",
		extensionNameInputSchema,
		toolspkg.RiskRead,
		true,
		false,
		[]string{extensionsExtensionsKey, extensionsStatusKey, "provenance"},
		[]string{"extension provenance", "extension integrity"},
	),
	nativeInteractiveOpenWorldExtensionDescriptor(
		toolspkg.ToolIDExtensionsPublish,
		"extensions_publish",
		"Extensions Publish",
		"Publish an immutable extension generation to GitHub Releases.",
		extensionPublishInputSchema,
		[]string{extensionsExtensionsKey, extensionsAuthoringKey, "publish"},
		[]string{"publish extension", "create extension release"},
	),
}

func nativeInteractiveOpenWorldExtensionDescriptor(
	id toolspkg.ToolID,
	nativeName string,
	title string,
	description string,
	inputSchema string,
	tags []string,
	searchHints []string,
) toolspkg.Descriptor {
	descriptor := nativeDescriptor(
		id,
		nativeName,
		title,
		description,
		inputSchema,
		toolspkg.RiskOpenWorld,
		false,
		false,
		true,
		[]toolspkg.ToolsetID{toolspkg.ToolsetIDExtensions},
		tags,
		searchHints,
	)
	descriptor.RequiresInteraction = true
	return descriptor
}

const extensionSearchInputSchema = `{
	"type":"object",
	"properties":{
		"query":{"type":"string"},
		"sources":{"type":"array","items":{"type":"string","enum":["curated","github"]},"uniqueItems":true},
		"limit":{"type":"integer","minimum":1,"maximum":100},
		"cursor":{"type":"string"}
	},
	"additionalProperties":false
}`

const extensionPublishInputSchema = `{
	"type":"object",
	"required":["generation_dir","repository","tag_name"],
	"properties":{
		"generation_dir":{"type":"string","minLength":1},
		"repository":{"type":"string","minLength":1},
		"tag_name":{"type":"string","minLength":1},
		"draft":{"type":"boolean"}
	},
	"additionalProperties":false
}`
