// entrypoint.ts
// Compozy entrypoint file - exports all available tools

import { logTool } from "./log_tool.ts";

// Export tools with snake_case keys for Compozy runtime
export default {
    "log_tool": logTool,
}
