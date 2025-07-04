// entrypoint.ts
// Compozy entrypoint file - exports all available tools

import { weatherTool } from "./weather_tool.ts";
import { saveDataTool } from "./save_data_tool.ts";

// Export tools with snake_case keys for Compozy runtime
export default {
    "weather_tool": weatherTool,
    "save_data": saveDataTool,
}
