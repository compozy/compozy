# useLocalRuntime

In-browser chat with custom model adapter.

## Basic Usage

```tsx
import { useLocalRuntime, AssistantRuntimeProvider } from "@assistant-ui/react";
import { Thread } from "@/components/assistant-ui/thread";

function App() {
  const runtime = useLocalRuntime({
    model: {
      async run({ messages, abortSignal }) {
        const response = await fetch("/api/chat", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ messages }),
          signal: abortSignal,
        });

        const data = await response.json();
        return {
          content: [{ type: "text", text: data.text }],
        };
      },
    },
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```

## Streaming Response

Use a generator and emit `ChatModelRunResult` chunks (append-only content parts):

```tsx
const runtime = useLocalRuntime({
  model: {
    async *run({ messages, abortSignal }) {
      const response = await fetch("/api/chat", {
        method: "POST",
        body: JSON.stringify({ messages }),
        signal: abortSignal,
      });

      const reader = response.body?.getReader();
      const decoder = new TextDecoder();
      let buffer = "";

      while (reader) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });

        // Split on newlines for this plain-text example (not Data Stream)
        const parts = buffer.split("\n");
        buffer = parts.pop() ?? "";

        for (const textChunk of parts.filter(Boolean)) {
          yield {
            content: [{ type: "text", text: textChunk }],
          };
        }
      }

      if (buffer) {
        yield { content: [{ type: "text", text: buffer }] };
      }
    },
  },
});
```

## Options

```tsx
interface LocalRuntimeOptions {
  model: ChatModelAdapter;
  initialMessages?: ThreadMessage[];
  adapters?: {
    attachments?: AttachmentAdapter;
    feedback?: FeedbackAdapter;
    speech?: SpeechSynthesisAdapter;
  };
}
```

## ChatModelAdapter

```tsx
interface ChatModelAdapter {
  run(options: ChatModelRunOptions): Promise<ChatModelRunResult> | AsyncGenerator<ChatModelRunResult>;
}

interface ChatModelRunOptions {
  messages: ThreadMessage[];
  abortSignal: AbortSignal;
  config?: Record<string, unknown>;
}

type ChatModelRunResult =
  | ChatModelRunResultFinal
  | ChatModelRunResultStream;

interface ChatModelRunResultFinal {
  content: MessagePart[];
}

// Streamed chunks are ChatModelRunResult objects
type ChatModelRunResultStream = ChatModelRunResult;
```

## With OpenAI Direct

```tsx
import OpenAI from "openai";

const openai = new OpenAI({
  apiKey: process.env.NEXT_PUBLIC_OPENAI_API_KEY,
  dangerouslyAllowBrowser: true, // Only for demos
});

const runtime = useLocalRuntime({
  model: {
    async *run({ messages, abortSignal }) {
      const stream = await openai.chat.completions.create({
        model: "gpt-4o",
        messages: messages.map((m) => ({
          role: m.role,
          content: m.content
            .filter((p): p is { type: "text"; text: string } => p.type === "text")
            .map((p) => p.text)
            .join(""),
        })),
        stream: true,
      });

      for await (const chunk of stream) {
        if (abortSignal.aborted) break;
        const delta = chunk.choices[0]?.delta?.content;
        if (delta) {
          yield { content: [{ type: "text", text: delta }] };
        }
      }
    },
  },
});
```

## With Tools

Emit tool calls as message parts (`type: "tool-call"`) and include `argsText` plus optional `result`:

```tsx
const runtime = useLocalRuntime({
  model: {
    async *run({ messages, abortSignal }) {
      const toolCallId = "1";

      // Yield tool call with parsed arguments
      yield {
        content: [
          {
            type: "tool-call",
            toolCallId,
            toolName: "get_weather",
            args: { city: "NYC" },
            argsText: '{"city":"NYC"}',
          },
        ],
      };

      // Execute tool
      const result = await getWeather({ city: "NYC" });

      // Send result on the same tool-call part
      yield {
        content: [
          {
            type: "tool-call",
            toolCallId,
            toolName: "get_weather",
            args: { city: "NYC" },
            argsText: '{"city":"NYC"}',
            result,
          },
          { type: "text", text: `The weather in NYC is ${result.temp}°C` },
        ],
      };
    },
  },
});
```

## With Attachments

```tsx
const runtime = useLocalRuntime({
  model: {
    async run({ messages }) {
      // Access attachments from last message
      const lastMessage = messages[messages.length - 1];
      const attachments = lastMessage.attachments || [];

      // Process attachments
      for (const attachment of attachments) {
        if (attachment.type === "image") {
          // Handle image
        }
      }

      return { content: [{ type: "text", text: "Processed" }] };
    },
  },
  adapters: {
    attachments: {
      accept: "image/*,application/pdf",
      async add({ file }) {
        const url = URL.createObjectURL(file);
        return {
          id: crypto.randomUUID(),
          name: file.name,
          type: file.type.startsWith("image/") ? "image" : "file",
          url,
        };
      },
      async send(attachment) {
        return attachment;
      },
      async remove() {},
    },
  },
});
```

## With Initial Messages

```tsx
const runtime = useLocalRuntime({
  model: { ... },
  initialMessages: [
    {
      id: "1",
      role: "assistant",
      content: [{ type: "text", text: "Hello! How can I help you?" }],
      status: "complete",
      createdAt: new Date(),
    },
  ],
});
```

## Error Handling

```tsx
const runtime = useLocalRuntime({
  model: {
    async *run({ messages, abortSignal }) {
      try {
        const response = await fetch("/api/chat", {
          method: "POST",
          body: JSON.stringify({ messages }),
          signal: abortSignal,
        });

        if (!response.ok) {
          throw new Error(`HTTP ${response.status}`);
        }

        // ... process response
      } catch (error) {
        if (error.name === "AbortError") {
          // User cancelled - normal, don't throw
          return;
        }
        throw error; // Re-throw to show error in UI
      }
    },
  },
});
```
