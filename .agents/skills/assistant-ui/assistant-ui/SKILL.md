---
name: assistant-ui
description: Guide for assistant-ui library - AI chat UI components. Use when asking about architecture, debugging, or understanding the codebase.
version: 0.0.1
license: MIT
---

# assistant-ui

**Always consult [assistant-ui.com/llms.txt](https://assistant-ui.com/llms.txt) for latest API.**

React library for building AI chat interfaces with composable primitives.

## References

- [./references/architecture.md](./references/architecture.md) -- Core architecture and layered system
- [./references/packages.md](./references/packages.md) -- Package overview and selection guide

## When to Use

| Use Case | Best For |
|----------|----------|
| Chat UI from scratch | Full control over UX |
| Existing AI backend | Connects to any streaming backend |
| Custom message types | Tools, images, files, custom parts |
| Multi-thread apps | Built-in thread list management |
| Production apps | Cloud persistence, auth, analytics |

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  UI Components (Primitives)             │
│    ThreadPrimitive, MessagePrimitive, ComposerPrimitive │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                   Context Hooks                         │
│   useAui, useAuiState, useAuiEvent │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                    Runtime Layer                        │
│  AssistantRuntime → ThreadRuntime → MessageRuntime      │
└─────────────────────────┬───────────────────────────────┘
                          │
┌─────────────────────────▼───────────────────────────────┐
│                   Adapters/Backend                      │
│   AI SDK · LangGraph · Custom · Cloud Persistence       │
└─────────────────────────────────────────────────────────┘
```

## Pick a Runtime

```
Using AI SDK?
├─ Yes → useChatRuntime (recommended)
└─ No
   ├─ External state (Redux/Zustand)? → useExternalStoreRuntime
   ├─ LangGraph agent? → useLangGraphRuntime
   ├─ AG-UI protocol? → useAgUiRuntime
   ├─ A2A protocol? → useA2ARuntime
   └─ Custom API → useLocalRuntime
```

## Core Packages

| Package | Purpose |
|---------|---------|
| `@assistant-ui/react` | UI primitives & hooks |
| `@assistant-ui/react-ai-sdk` | Vercel AI SDK v6 adapter |
| `@assistant-ui/react-langgraph` | LangGraph adapter |
| `@assistant-ui/react-markdown` | Markdown rendering |
| `assistant-stream` | Streaming protocol |
| `assistant-cloud` | Cloud persistence |

## Quick Start

```tsx
import { AssistantRuntimeProvider } from "@assistant-ui/react";
import { Thread } from "@/components/assistant-ui/thread";
import { useChatRuntime, AssistantChatTransport } from "@assistant-ui/react-ai-sdk";

function App() {
  const runtime = useChatRuntime({
    transport: new AssistantChatTransport({ api: "/api/chat" }),
  });
  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```

## State Access

```tsx
import { useAui, useAuiState } from "@assistant-ui/react";

const api = useAui();
api.thread().append({ role: "user", content: [{ type: "text", text: "Hi" }] });
api.thread().cancelRun();

const messages = useAuiState(s => s.thread.messages);
const isRunning = useAuiState(s => s.thread.isRunning);
```

## Related Skills

- `/setup` - Installation and configuration
- `/primitives` - UI component customization
- `/runtime` - State management deep dive
- `/tools` - Tool registration and UI
- `/streaming` - Streaming protocols
- `/cloud` - Persistence and auth
- `/thread-list` - Multi-thread management
- `/update` - Version updates and migrations
