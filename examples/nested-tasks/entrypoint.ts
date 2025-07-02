// entrypoint.ts
// Compozy entrypoint file - exports all available tools

import { counterTool } from "./counter_tool.ts";
import { echoTool } from "./echo_tool.ts";

// Export tools with snake_case keys for Compozy runtime
export default {
    "counter_tool": counterTool,
    "echo_tool": echoTool,
}
