---
name: assistant-ui-tools
description: Guide for tool registration and tool UI in assistant-ui. Use when implementing LLM tools, tool call rendering, or human-in-the-loop patterns.
version: 0.0.1
license: MIT
---

# assistant-ui Tools

**Always consult [assistant-ui.com/llms.txt](https://assistant-ui.com/llms.txt) for latest API.**

Tools let LLMs trigger actions with custom UI rendering.

## References

- [./references/make-tool.md](./references/make-tool.md) -- makeAssistantTool/useAssistantTool
- [./references/tool-ui.md](./references/tool-ui.md) -- makeAssistantToolUI rendering
- [./references/human-in-loop.md](./references/human-in-loop.md) -- Confirmation patterns

## Tool Types

```
Where does the tool execute?
├─ Backend (LLM calls API) → AI SDK tool()
│  └─ Want custom UI? → makeAssistantToolUI
└─ Frontend (browser-only) → makeAssistantTool
   └─ Want custom UI? → makeAssistantToolUI
```

## Backend Tool with UI

```ts
// Backend (app/api/chat/route.ts)
import { tool, stepCountIs } from "ai";
import { z } from "zod";

const tools = {
  get_weather: tool({
    description: "Get weather for a city",
    inputSchema: z.object({ city: z.string() }),
    execute: async ({ city }) => ({ temp: 22, city }),
  }),
};

const result = streamText({
  model: openai("gpt-4o"),
  messages,
  tools,
  stopWhen: stepCountIs(5),
});
```

```tsx
// Frontend
import { makeAssistantToolUI } from "@assistant-ui/react";

const WeatherToolUI = makeAssistantToolUI({
  toolName: "get_weather",
  render: ({ args, result, status }) => {
    if (status === "running") return <div>Loading weather...</div>;
    return <div>{result?.city}: {result?.temp}°C</div>;
  },
});

// Register in app
<AssistantRuntimeProvider runtime={runtime}>
  <WeatherToolUI />
  <Thread />
</AssistantRuntimeProvider>
```

## Frontend-Only Tool

```tsx
import { makeAssistantTool } from "@assistant-ui/react";
import { z } from "zod";

const CopyTool = makeAssistantTool({
  toolName: "copy_to_clipboard",
  parameters: z.object({ text: z.string() }),
  execute: async ({ text }) => {
    await navigator.clipboard.writeText(text);
    return { success: true };
  },
});

<AssistantRuntimeProvider runtime={runtime}>
  <CopyTool />
  <Thread />
</AssistantRuntimeProvider>
```

## API Reference

```tsx
// makeAssistantToolUI props
interface ToolUIProps {
  toolCallId: string;
  toolName: string;
  args: Record<string, unknown>;
  argsText: string;
  result?: unknown;
  status: "running" | "complete" | "incomplete" | "requires-action";
  submitResult: (result: unknown) => void;  // For interactive tools
}
```

## Human-in-the-Loop

```tsx
const DeleteToolUI = makeAssistantToolUI({
  toolName: "delete_file",
  render: ({ args, status, submitResult }) => {
    if (status === "requires-action") {
      return (
        <div>
          <p>Delete {args.path}?</p>
          <button onClick={() => submitResult({ confirmed: true })}>Confirm</button>
          <button onClick={() => submitResult({ confirmed: false })}>Cancel</button>
        </div>
      );
    }
    return <div>File deleted</div>;
  },
});
```

## Common Gotchas

**Tool UI not rendering**
- `toolName` must match exactly (case-sensitive)
- Register UI inside `AssistantRuntimeProvider`

**Tool not being called**
- Check tool description is clear
- Use `stopWhen: stepCountIs(n)` to allow multi-step

**Result not showing**
- Tool must return a value
- Check `status === "complete"` before accessing result
