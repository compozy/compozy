package events

var mcpAuthRegistryEntries = []Metadata{
	global(info(MCPAuthBegin, "mcp.auth", ComponentMCP)),
	global(info(MCPAuthExchange, "mcp.auth", ComponentMCP)),
	global(info(MCPAuthLogout, "mcp.auth", ComponentMCP)),
}

var registryEntries = append(
	append(append([]Metadata{}, baseRegistryEntries...), mcpAuthRegistryEntries...),
	networkCoordinationRegistryEntries...,
)
