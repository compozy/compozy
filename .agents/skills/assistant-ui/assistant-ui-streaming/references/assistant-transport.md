# Assistant Transport Format

assistant-ui's native streaming protocol.

## Overview

Assistant Transport is assistant-ui's optimized format with support for all features including reasoning, sources, and complex message structures.

## When to Use

- Custom backends not using AI SDK
- Need all assistant-ui features
- Building your own streaming server

## Usage

### Server

```ts
import {
  AssistantStream,
  AssistantTransportEncoder,
  createAssistantStreamController,
} from "assistant-stream";

export async function POST(req: Request) {
  const [stream, controller] = createAssistantStreamController();

  controller.appendText("Hello ");
  controller.appendText("world!");
  controller.close();

  return AssistantStream.toResponse(stream, new AssistantTransportEncoder());
}
```

### Client

```ts
import { AssistantStream, AssistantTransportDecoder } from "assistant-stream";

const stream = AssistantStream.fromResponse(
  response,
  new AssistantTransportDecoder()
);

for await (const chunk of stream) {
  console.log(chunk);
}
```

## Event Types

### Chunk Shapes

AssistantStream chunks (decoded from AssistantTransport) match the core types:

- `part-start` with `part.type` = `"text" | "reasoning" | "tool-call" | "source" | "file"`
- `part-finish` and `tool-call-args-text-finish`
- `text-delta` / `annotations` / `data`
- `result` (tool results)
- `step-start` / `step-finish` / `message-finish`
- `error`

## Complete Example

```ts
import {
  AssistantStream,
  AssistantTransportEncoder,
  createAssistantStreamController,
} from "assistant-stream";

async function streamResponse(query: string) {
  const [stream, controller] = createAssistantStreamController();
  const toolCallId = `tool_${Date.now()}`;

  controller.appendText("Based on my search, ");

  const tool = controller.addToolCallPart({
    toolCallId,
    toolName: "search",
  });
  tool.argsText.append(JSON.stringify({ query }));
  tool.argsText.close();

  const searchResult = await performSearch(query);
  tool.setResponse({ result: searchResult });

  controller.appendText(`here's what I found:\n\n${searchResult.summary}`);
  controller.close();

  return AssistantStream.toResponse(stream, new AssistantTransportEncoder());
}
```

## Using with useLocalRuntime

```tsx
import { useLocalRuntime } from "@assistant-ui/react";
import { AssistantStream, AssistantTransportDecoder } from "assistant-stream";

const runtime = useLocalRuntime({
  model: {
    async *run({ messages, abortSignal }) {
      const response = await fetch("/api/chat", {
        method: "POST",
        body: JSON.stringify({ messages }),
        signal: abortSignal,
      });

      let currentTool:
        | {
            toolCallId: string;
            toolName: string;
            args: Record<string, unknown>;
            argsText: string;
          }
        | undefined;

      const stream = AssistantStream.fromResponse(
        response,
        new AssistantTransportDecoder()
      );

      for await (const chunk of stream) {
        // Convert AssistantStreamChunk into ChatModelRunResult content parts
        if (chunk.type === "text-delta") {
          yield { content: [{ type: "text", text: chunk.textDelta }] };
        }

        // Track current tool-call to attach result to it
        if (chunk.type === "part-start" && chunk.part.type === "tool-call") {
          currentTool = {
            toolCallId: chunk.part.toolCallId,
            toolName: chunk.part.toolName,
            args: {},
            argsText: "{}",
          };
          yield { content: [currentTool] };
        }

        if (chunk.type === "result" && currentTool) {
          yield {
            content: [
              {
                ...currentTool,
                result: chunk.result,
                artifact: chunk.artifact,
                isError: chunk.isError,
              },
            ],
          };
          currentTool = undefined;
        }
      }
    },
  },
});
```
