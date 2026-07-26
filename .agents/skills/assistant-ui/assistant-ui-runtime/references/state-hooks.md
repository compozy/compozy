# State Hooks

Accessing assistant-ui runtime state.

## Modern API (Recommended)

### useAui

Get the runtime API for imperative actions.

```tsx
import { useAui } from "@assistant-ui/react";

function Controls() {
  const api = useAui();

  // Thread operations
  const thread = api.thread();
  thread.append({ role: "user", content: [{ type: "text", text: "Hi" }] });
  thread.cancelRun();
  thread.startRun();

  // Message operations
  const message = thread.message(0);
  message.edit({ ... });
  message.reload();

  // Thread list operations
  const threads = api.threads();
  threads.switchToThread(threadId);
  threads.switchToNewThread();

  // Get state snapshot
  const state = api.getState();
}
```

### useAuiState

Subscribe to state changes with a selector.

```tsx
import { useAuiState } from "@assistant-ui/react";

function MessageCount() {
  // Re-renders when messages change
  const messages = useAuiState((s) => s.thread.messages);
  return <div>{messages.length} messages</div>;
}

function RunningIndicator() {
  // Only re-renders when isRunning changes
  const isRunning = useAuiState((s) => s.thread.isRunning);
  return isRunning ? <Spinner /> : null;
}

function ComposerText() {
  const text = useAuiState((s) => s.thread.composer.text);
  return <div>Typing: {text}</div>;
}

function ThreadInfo() {
  // Multiple values
  const { messages, isRunning, capabilities } = useAuiState((s) => ({
    messages: s.thread.messages,
    isRunning: s.thread.isRunning,
    capabilities: s.thread.capabilities,
  }));
}
```

### useAuiEvent

Listen to runtime events.

```tsx
import { useAuiEvent } from "@assistant-ui/react";

function Analytics() {
  useAuiEvent("composer.send", (event) => {
    analytics.track("message_sent", {
      threadId: event.threadId,
      messageId: event.messageId, // optional, may be undefined
    });
  });

  useAuiEvent("thread.runStart", () => {
    console.log("Generation started");
  });

  useAuiEvent("thread.runEnd", () => {
    console.log("Generation completed");
  });

  return null;
}
```

Available events:
- `composer.send` - Message submitted from composer
- `composer.attachmentAdd` - Attachment added in composer
- `thread.runStart` - Generation started
- `thread.runEnd` - Generation ended
- `thread.initialize` - Thread is initialized
- `thread.modelContextUpdate` - Thread model context updated
- `threadListItem.switchedTo` - Active thread changed
- `threadListItem.switchedAway` - Active thread changed away

## State Shape

```typescript
interface AssistantState {
  thread: {
    messages: ThreadMessage[];
    isRunning: boolean;
    capabilities: RuntimeCapabilities;
    composer: {
      text: string;
      attachments: Attachment[];
    };
  };
  threads: {
    mainThreadId: string;
    newThreadId: string | null;
    threadIds: readonly string[];
    archivedThreadIds: readonly string[];
    isLoading: boolean;
    threadItems: readonly ThreadListItemState[];
    main: ThreadState;
  };
  threadListItem: {
    id: string;
    remoteId?: string;
    externalId?: string;
    title?: string;
    status: "archived" | "regular" | "new" | "deleted";
  };
}
```

## Legacy Hooks

These still work but prefer the modern API:

```tsx
// Runtime access
import {
  useAssistantRuntime,
  useThreadRuntime,
  useMessageRuntime,
  useComposerRuntime,
} from "@assistant-ui/react";

const assistantRuntime = useAssistantRuntime();
const threadRuntime = useThreadRuntime();
const messageRuntime = useMessageRuntime();  // Needs message context
const composerRuntime = useComposerRuntime();

// State subscriptions
import {
  useThread,
  useThreadMessages,
  useComposer,
  useMessage,
  useThreadList,
} from "@assistant-ui/react";

const thread = useThread();           // { messages, isRunning, ... }
const messages = useThreadMessages(); // ThreadMessage[]
const composer = useComposer();       // { text, attachments, ... }
const message = useMessage();         // Current message (needs context)
const threadList = useThreadList();   // Thread list state
```

## Context Requirements

Some hooks require being inside specific contexts:

```tsx
// These work anywhere inside AssistantRuntimeProvider
useAui()
useAuiState()
useAuiEvent()
useAssistantRuntime()
useThreadRuntime()
useThread()
useThreadMessages()
useComposer()

// These require message context (inside ThreadPrimitive.Messages)
useMessageRuntime()
useMessage()

// These require message part context
useMessagePartRuntime()
```

## Performance Tips

### Use Selectors

```tsx
// Bad - re-renders on any state change
const state = useAuiState((s) => s);

// Good - only re-renders when messages change
const messages = useAuiState((s) => s.thread.messages);

// Better - only re-renders when message count changes
const count = useAuiState((s) => s.thread.messages.length);
```

### Memoize Derived Data

```tsx
function MessageList() {
  const messages = useAuiState((s) => s.thread.messages);

  // Memoize expensive computations
  const userMessages = useMemo(
    () => messages.filter((m) => m.role === "user"),
    [messages]
  );

  return <div>{userMessages.length} user messages</div>;
}
```

### Split Components

```tsx
// Bad - entire component re-renders
function Chat() {
  const messages = useAuiState((s) => s.thread.messages);
  const isRunning = useAuiState((s) => s.thread.isRunning);
  return (
    <div>
      <MessageList messages={messages} />
      <RunningIndicator running={isRunning} />
    </div>
  );
}

// Good - components re-render independently
function Chat() {
  return (
    <div>
      <MessageList />
      <RunningIndicator />
    </div>
  );
}

function MessageList() {
  const messages = useAuiState((s) => s.thread.messages);
  return <div>...</div>;
}

function RunningIndicator() {
  const isRunning = useAuiState((s) => s.thread.isRunning);
  return isRunning ? <Spinner /> : null;
}
```

## Direct Subscription

For non-React contexts:

```tsx
const api = useAui();

useEffect(() => {
  const runtime = api.thread();

  // Subscribe to changes
  const unsubscribe = runtime.subscribe(() => {
    const state = runtime.getState();
    console.log("State changed:", state);
  });

  return unsubscribe;
}, [api]);
```
