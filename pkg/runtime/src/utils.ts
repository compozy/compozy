import type { Logger } from "./logger.ts";
import type { ToolRequest } from "./types.ts";

export interface Tool {
  run<I, O>(input: I): O;
}

export async function loadToolDinamically(tool_id: string): Promise<Tool> {
  const file = await import(`${tool_id}`);
  if (!file.run) {
    throw new Error(`Tool ${tool_id} does not have a run function`);
  }
  return file;
}

export async function loadToolsDinamically(
  tools: ToolRequest[],
  logger: Logger,
) {
  const fnMaps = new Map();
  for (const tool of tools) {
    const { tool_id } = tool;
    try {
      const file = await loadToolDinamically(tool_id);
      fnMaps.set(tool_id, file);
    } catch (error: any) {
      logger.error(`Failed to import tool ${tool_id}`, {
        error: error.message,
      });
      throw error;
    }
  }
  return fnMaps;
}
