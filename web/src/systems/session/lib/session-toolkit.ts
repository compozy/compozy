import type { ToolDefinition, Toolkit } from "@assistant-ui/react";

// Every session tool executes server-side: the daemon streams each call and its
// result as already-resolved transcript parts, so the client registers these
// tools only to mark them backend-owned. Tool rendering is centralized in the
// session timeline's `ToolCallRow` (the single fallback renderer), so no toolkit
// entry carries a per-tool `render` — one shared backend definition is reused for
// every registered tool.
const backendTool: ToolDefinition = { type: "backend" };

export const sessionToolkit: Toolkit = {
  Bash: backendTool,
  Read: backendTool,
  Write: backendTool,
  Edit: backendTool,
  Grep: backendTool,
  Glob: backendTool,
};
