# assistant-ui Architecture

## Layered System

assistant-ui follows a 4-layer architecture where each layer depends only on layers below it.

### Layer 1: RuntimeCore (Internal)

Internal implementations that manage state:

- `LocalRuntimeCore` - In-browser state
- `ExternalStoreRuntimeCore` - External state sync
- `ThreadListRuntimeCore` - Thread management

```typescript
// Internal - not directly used
interface ThreadRuntimeCore {
  readonly messages: readonly ThreadMessage[];
  readonly isRunning: boolean;
  append(message: AppendMessage): void;
  cancelRun(): void;
  subscribe(callback: () => void): Unsubscribe;
}
```

### Layer 2: Runtime (Public API)

Public API exposed via hooks:

```typescript
type AssistantRuntime = {
  thread(): ThreadRuntime;
  threads(): ThreadListRuntime;
  getState(): AssistantState;
  subscribe(callback: () => void): Unsubscribe;
};

type ThreadRuntime = {
  getState(): ThreadState;
  append(message: AppendMessage): void;
  cancelRun(): void;
  message(index: number): MessageRuntime;
  composer(): ComposerRuntime;
};

type MessageRuntime = {
  getState(): MessageState;
  edit(message: EditMessage): void;
  reload(): void;
  part(index: number): MessagePartRuntime;
};
```

### Layer 3: Context Hooks

React hooks for accessing runtime:

```tsx
// Modern API (recommended)
import { useAui, useAuiState, useAuiEvent } from "@assistant-ui/react";

// Get API for imperative actions
const api = useAui();

// Subscribe to state changes
const messages = useAuiState(s => s.thread.messages);

// Listen to events
useAuiEvent("composer.send", (e) => console.log(e));
```

### Layer 4: Primitives (UI)

Composable UI components:

```tsx
import {
  ThreadPrimitive,
  ComposerPrimitive,
  MessagePrimitive,
  ActionBarPrimitive,
} from "@assistant-ui/react";
```

## Data Flow

```
User Action (send message)
    │
    ▼
Primitive captures event
    │
    ▼
Calls runtime API (thread.append)
    │
    ▼
RuntimeCore processes action
    │
    ▼
State updates
    │
    ▼
Subscribers notified
    │
    ▼
Primitives re-render with new state
```

## Message Model

```typescript
type ThreadMessage =
  | ThreadUserMessage
  | ThreadAssistantMessage
  | ThreadSystemMessage;

interface ThreadUserMessage {
  id: string;
  role: "user";
  content: MessagePart[];
  attachments?: Attachment[];
  createdAt: Date;
}

interface ThreadAssistantMessage {
  id: string;
  role: "assistant";
  content: MessagePart[];
  status: "running" | "complete" | "incomplete" | "requires-action";
  createdAt: Date;
}

type MessagePart =
  | { type: "text"; text: string }
  | { type: "image"; image: string }
  | {
      type: "tool-call";
      toolCallId: string;
      toolName: string;
      args: unknown;
      argsText: string;
      result?: unknown;
      isError?: boolean;
      artifact?: unknown;
    }
  | { type: "reasoning"; text: string }
  | {
      type: "source";
      sourceType: "url";
      id: string;
      url: string;
      title?: string;
    }
  | {
      type: "file";
      filename?: string;
      data: string;
      mimeType: string;
    };
```

## Branching Model

Messages form a tree structure supporting edits:

```
User: "Hello"
    └─ Assistant: "Hi there!"
       └─ User: "Tell me a joke"          ← Current branch
          └─ Assistant: "Why did..."
       └─ User: "Tell me a fact" (edit)   ← Alternative branch
          └─ Assistant: "The sun..."
```

Navigate branches with `BranchPickerPrimitive` or runtime API.
