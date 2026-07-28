# Thread List Runtime

Managing multiple chat threads.

## Overview

Thread list features are automatically available when using `useChatRuntime` with cloud persistence or `unstable_useRemoteThreadListRuntime`.

## ThreadListRuntime API

```typescript
type ThreadListRuntime = {
  getState(): ThreadListState;
  subscribe(callback: () => void): Unsubscribe;

  main: ThreadRuntime;              // Current active thread
  getById(threadId: string): ThreadRuntime;

  mainItem: ThreadListItemRuntime;  // Current thread item
  getItemById(threadId: string): ThreadListItemRuntime;
  getItemByIndex(idx: number): ThreadListItemRuntime;
  getArchivedItemByIndex(idx: number): ThreadListItemRuntime;

  switchToThread(threadId: string): Promise<void>;
  switchToNewThread(): Promise<void>;
};
```

## ThreadListState

This is the state shape returned by `ThreadListRuntime.getState()` (runtime API).
For app-level state via `useAuiState((s) => s.threads)`, use the client `ThreadsState` shape (`newThreadId: string | null`, `threadItems: readonly ThreadListItemState[]`).

```typescript
type ThreadListState = {
  mainThreadId: string;              // Current thread ID
  newThreadId: string | undefined;     // Pending new thread ID
  threadIds: readonly string[];        // Regular thread IDs
  archivedThreadIds: readonly string[];
  isLoading: boolean;
  threadItems: Record<string, Omit<ThreadListItemState, "isMain" | "threadId">>;
};
```

## ThreadListItemRuntime API

```typescript
type ThreadListItemRuntime = {
  getState(): ThreadListItemState;

  switchTo(): Promise<void>;
  rename(newTitle: string): Promise<void>;
  archive(): Promise<void>;
  unarchive(): Promise<void>;
  delete(): Promise<void>;

  initialize(): Promise<{ remoteId: string; externalId?: string }>;
  generateTitle(): Promise<void>;

  subscribe(callback: () => void): Unsubscribe;
};
```

## Accessing Thread List

```tsx
import { useAui, useAuiState } from "@assistant-ui/react";

function ThreadListComponent() {
  const api = useAui();

  // Get thread list state
  const { threadIds, archivedThreadIds, isLoading } = useAuiState(
    (s) => s.threads
  );

  // Switch threads
  const handleSwitch = (threadId: string) => {
    api.threads().switchToThread(threadId);
  };

  // Create new thread
  const handleNew = () => {
    api.threads().switchToNewThread();
  };

  return (
    <div>
      <button onClick={handleNew}>New Chat</button>
      {threadIds.map((threadId) => (
        <button key={threadId} onClick={() => handleSwitch(threadId)}>
          {threadId}
        </button>
      ))}
    </div>
  );
}
```

## Thread Item Operations

```tsx
function ThreadItem({ threadId }: { threadId: string }) {
  const api = useAui();
  const item = api.threads().item({ id: threadId });

  const handleRename = async () => {
    await item.rename("New Title");
  };

  const handleArchive = async () => {
    await item.archive();
  };

  const handleDelete = async () => {
    await item.delete();
  };

  return (
    <div>
      <button onClick={() => item.switchTo()}>Switch</button>
      <button onClick={handleRename}>Rename</button>
      <button onClick={handleArchive}>Archive</button>
      <button onClick={handleDelete}>Delete</button>
    </div>
  );
}
```

## Using ThreadList Primitives

```tsx
import {
  ThreadListPrimitive,
  ThreadListItemPrimitive,
} from "@assistant-ui/react";

function ThreadList() {
  return (
    <ThreadListPrimitive.Root className="w-64 border-r p-2">
      <ThreadListPrimitive.New className="w-full p-2 mb-2 bg-blue-500 text-white rounded">
        + New Chat
      </ThreadListPrimitive.New>

      <div className="space-y-1">
        <ThreadListPrimitive.Items>
          <ThreadListItemPrimitive.Root className="flex items-center p-2 hover:bg-gray-100 rounded group">
            <ThreadListItemPrimitive.Trigger className="flex-1 text-left truncate">
              <ThreadListItemPrimitive.Title />
            </ThreadListItemPrimitive.Trigger>

            <div className="hidden group-hover:flex gap-1">
              <ThreadListItemPrimitive.Archive className="p-1 text-gray-500 hover:text-gray-700">
                📁
              </ThreadListItemPrimitive.Archive>
              <ThreadListItemPrimitive.Delete className="p-1 text-red-500 hover:text-red-700">
                🗑️
              </ThreadListItemPrimitive.Delete>
            </div>
          </ThreadListItemPrimitive.Root>
        </ThreadListPrimitive.Items>
      </div>
    </ThreadListPrimitive.Root>
  );
}
```

## With Custom Thread List

```tsx
function SidebarWithThreadList() {
  const { threadIds, mainThreadId } = useAuiState((s) => ({
    threadIds: s.threads.threadIds,
    mainThreadId: s.threads.mainThreadId,
  }));
  const api = useAui();

  return (
    <aside className="w-64 bg-gray-50 h-full">
      <div className="p-4">
        <button
          onClick={() => api.threads().switchToNewThread()}
          className="w-full p-2 bg-blue-500 text-white rounded"
        >
          New Chat
        </button>
      </div>

      <nav className="p-2">
        {threadIds.map((threadId) => {
          const isActive = threadId === mainThreadId;
          return (
            <button
              key={threadId}
              onClick={() => api.threads().switchToThread(threadId)}
              className={`w-full p-2 text-left rounded ${
                isActive ? "bg-blue-100" : "hover:bg-gray-100"
              }`}
            >
              {threadId.slice(0, 8)}...
            </button>
          );
        })}
      </nav>
    </aside>
  );
}
```

## Remote Thread List Adapter

For custom persistence:

```tsx
import {
  AssistantRuntimeProvider,
  unstable_useRemoteThreadListRuntime as useRemoteThreadListRuntime,
  useLocalRuntime,
} from "@assistant-ui/react";
import { Thread } from "@/components/assistant-ui/thread";
import { ThreadList } from "@/components/assistant-ui/thread-list";

const adapter: RemoteThreadListAdapter = {
  async list() {
    const threads = await api.getThreads();
    return {
      threads: threads.map((t) => ({
        remoteId: t.id,
        status: t.archived ? "archived" : "regular",
        title: t.title,
      })),
    };
  },

  async initialize(threadId) {
    const thread = await api.createThread({ localId: threadId });
    return { remoteId: thread.id };
  },

  async rename(remoteId, newTitle) {
    await api.updateThread(remoteId, { title: newTitle });
  },

  async archive(remoteId) {
    await api.updateThread(remoteId, { archived: true });
  },

  async unarchive(remoteId) {
    await api.updateThread(remoteId, { archived: false });
  },

  async delete(remoteId) {
    await api.deleteThread(remoteId);
  },

  async generateTitle(remoteId, messages) {
    return api.generateTitle(remoteId, messages);
  },

  async fetch(threadId) {
    const thread = await api.getThread(threadId);
    return {
      remoteId: thread.id,
      status: thread.archived ? "archived" : "regular",
      title: thread.title,
    };
  },
};

function App() {
  const runtime = useRemoteThreadListRuntime({
    adapter,
    runtimeHook: () => useLocalRuntime({ model: myModel }),
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <ThreadList />
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```
