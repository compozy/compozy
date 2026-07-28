---
name: assistant-ui-runtime
description: Guide for assistant-ui runtime system and state management. Use when working with runtimes, accessing state, or managing thread/message data.
version: 0.0.1
license: MIT
---

# assistant-ui Runtime

**Always consult [assistant-ui.com/llms.txt](https://assistant-ui.com/llms.txt) for latest API.**

## References

- [./references/local-runtime.md](./references/local-runtime.md) -- useLocalRuntime deep dive
- [./references/external-store.md](./references/external-store.md) -- useExternalStoreRuntime deep dive
- [./references/thread-list.md](./references/thread-list.md) -- Thread list management
- [./references/state-hooks.md](./references/state-hooks.md) -- State access hooks
- [./references/types.md](./references/types.md) -- Type definitions

## Runtime Hierarchy

```
AssistantRuntime
├── ThreadListRuntime (thread management)
│   ├── ThreadListItemRuntime (per-thread item)
│   └── ...
└── ThreadRuntime (current thread)
    ├── ComposerRuntime (input state)
    └── MessageRuntime[] (per-message)
        └── MessagePartRuntime[] (per-content-part)
```

## State Access (Modern API)

```tsx
import { useAui, useAuiState, useAuiEvent } from "@assistant-ui/react";

function ChatControls() {
  const api = useAui();
  const messages = useAuiState(s => s.thread.messages);
  const isRunning = useAuiState(s => s.thread.isRunning);

  useAuiEvent("composer.send", (e) => {
    console.log("Sent in thread:", e.threadId);
  });

  return (
    <div>
      <button onClick={() => api.thread().append({
        role: "user",
        content: [{ type: "text", text: "Hello!" }],
      })}>
        Send
      </button>
      {isRunning && (
        <button onClick={() => api.thread().cancelRun()}>Cancel</button>
      )}
    </div>
  );
}
```

## Thread Operations

```tsx
const api = useAui();
const thread = api.thread();

// Append message
thread.append({ role: "user", content: [{ type: "text", text: "Hello" }] });

// Cancel generation
thread.cancelRun();

// Get current state
const state = thread.getState();  // { messages, isRunning, ... }
```

## Message Operations

```tsx
const message = api.thread().message(0);  // By index

message.edit({ role: "user", content: [{ type: "text", text: "Updated" }] });
message.reload();
```

## Events

```tsx
useAuiEvent("thread.runStart", () => {});
useAuiEvent("thread.runEnd", () => {});
useAuiEvent("composer.send", ({ threadId }) => {
  console.log("Sent in thread:", threadId);
});
useAuiEvent("thread.modelContextUpdate", () => {});
```

## Capabilities

```tsx
const caps = useAuiState(s => s.thread.capabilities);
// { cancel, edit, reload, copy, speak, attachments }
```

## Quick Reference

```tsx
// Get messages
const messages = useAuiState(s => s.thread.messages);

// Check running state
const isRunning = useAuiState(s => s.thread.isRunning);

// Append message
api.thread().append({ role: "user", content: [{ type: "text", text: "Hi" }] });

// Cancel generation
api.thread().cancelRun();

// Edit message
api.thread().message(index).edit({ ... });

// Reload message
api.thread().message(index).reload();
```

## Common Gotchas

**"Cannot read property of undefined"**
- Ensure hooks are called inside `AssistantRuntimeProvider`

**State not updating**
- Use selectors with `useAuiState` to prevent unnecessary re-renders

**Messages array empty**
- Check runtime is configured
- Verify API response format
