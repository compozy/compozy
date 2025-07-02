// entrypoint.ts
// Compozy entrypoint file - exports all available tools

import { echoTool } from "./echo_tool.ts";

// Export tools with snake_case keys for Compozy runtime
export default {
    "echo_tool": echoTool,
}
