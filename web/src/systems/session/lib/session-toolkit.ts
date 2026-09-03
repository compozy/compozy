import type { ToolDefinition, Toolkit } from "@assistant-ui/react";

// Every session tool executes server-side: the daemon streams each call and its
// result as already-resolved transcript parts, so the client registers these
// tools only to mark them backend-owned. No toolkit entry carries a per-tool
// `render`. The timeline hosts specialized surfaces (a supervised terminal is
// its own block) and uses `ToolCallRow` only as the fallback for ordinary tools.
const backendTool: ToolDefinition = { type: "backend" };

export const sessionToolkit: Toolkit = {
  Bash: backendTool,
  Read: backendTool,
  Write: backendTool,
  Edit: backendTool,
  Grep: backendTool,
  Glob: backendTool,
  compozy__terminal_exec: backendTool,
  compozy__terminal_open: backendTool,
};
