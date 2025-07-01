// entrypoint.ts
// Compozy entrypoint file - exports all available tools

import { weatherTool } from "./weather_tool.ts";
import { saveTool } from "./save_tool.ts";

// Export tools with snake_case keys for Compozy runtime
export default {
    "weather_tool": weatherTool,
    "save_tool": saveTool,
}
