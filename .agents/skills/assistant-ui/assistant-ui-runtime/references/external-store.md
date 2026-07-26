# useExternalStoreRuntime

Connect assistant-ui to custom message stores (Redux, Zustand, etc.).

## Basic Usage

```tsx
import { useExternalStoreRuntime, AssistantRuntimeProvider } from "@assistant-ui/react";
import { Thread } from "@/components/assistant-ui/thread";

function App() {
  // Your existing state
  const [messages, setMessages] = useState<ThreadMessage[]>([]);
  const [isRunning, setIsRunning] = useState(false);

  const runtime = useExternalStoreRuntime({
    messages,
    isRunning,
    onNew: async (message) => {
      setMessages((prev) => [...prev, message]);
      setIsRunning(true);

      // Call your API
      const response = await fetch("/api/chat", {
        method: "POST",
        body: JSON.stringify({ messages: [...messages, message] }),
      });

      const data = await response.json();
      setMessages((prev) => [...prev, {
        id: crypto.randomUUID(),
        role: "assistant",
        content: [{ type: "text", text: data.text }],
        status: "complete",
        createdAt: new Date(),
      }]);
      setIsRunning(false);
    },
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```

## Options

```tsx
interface ExternalStoreRuntimeOptions<T = ThreadMessage> {
  // Required
  messages: readonly T[];
  isRunning: boolean;
  onNew: (message: AppendMessage) => Promise<void>;

  // Optional callbacks
  onEdit?: (message: AppendMessage) => Promise<void>;
  onReload?: (parentId: string | null) => Promise<void>;
  onCancel?: () => Promise<void>;

  // Message conversion (for custom message formats)
  convertMessage?: (message: T) => ThreadMessage;

  // Capabilities override
  capabilities?: Partial<RuntimeCapabilities>;

  // Adapters
  adapters?: {
    attachments?: AttachmentAdapter;
    feedback?: FeedbackAdapter;
    speech?: SpeechSynthesisAdapter;
  };
}
```

## With Redux

```tsx
import { useSelector, useDispatch } from "react-redux";
import { addMessage, setRunning } from "./chatSlice";

function Chat() {
  const dispatch = useDispatch();
  const messages = useSelector((state) => state.chat.messages);
  const isRunning = useSelector((state) => state.chat.isRunning);

  const runtime = useExternalStoreRuntime({
    messages,
    isRunning,
    onNew: async (message) => {
      dispatch(addMessage(message));
      dispatch(setRunning(true));

      const response = await chatAPI(message);

      dispatch(addMessage(response));
      dispatch(setRunning(false));
    },
    onEdit: async (message) => {
      dispatch(editMessage(message));
      // Re-generate response...
    },
    onReload: async (parentId) => {
      dispatch(regenerateFrom(parentId));
    },
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```

## With Zustand

```tsx
import { create } from "zustand";

interface ChatStore {
  messages: ThreadMessage[];
  isRunning: boolean;
  addMessage: (msg: ThreadMessage) => void;
  setRunning: (running: boolean) => void;
}

const useChatStore = create<ChatStore>((set) => ({
  messages: [],
  isRunning: false,
  addMessage: (msg) => set((s) => ({ messages: [...s.messages, msg] })),
  setRunning: (running) => set({ isRunning: running }),
}));

function Chat() {
  const { messages, isRunning, addMessage, setRunning } = useChatStore();

  const runtime = useExternalStoreRuntime({
    messages,
    isRunning,
    onNew: async (message) => {
      addMessage(message);
      setRunning(true);

      const response = await fetchChat(messages.concat(message));

      addMessage(response);
      setRunning(false);
    },
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```

## Custom Message Format

```tsx
// Your message format
interface MyMessage {
  uuid: string;
  sender: "human" | "ai";
  text: string;
  timestamp: number;
}

function Chat() {
  const [messages, setMessages] = useState<MyMessage[]>([]);

  const runtime = useExternalStoreRuntime({
    messages,
    isRunning: false,
    convertMessage: (msg: MyMessage): ThreadMessage => ({
      id: msg.uuid,
      role: msg.sender === "human" ? "user" : "assistant",
      content: [{ type: "text", text: msg.text }],
      status: "complete",
      createdAt: new Date(msg.timestamp),
    }),
    onNew: async (appendMessage) => {
      // Convert back to your format
      const myMessage: MyMessage = {
        uuid: crypto.randomUUID(),
        sender: "human",
        text: appendMessage.content
          .filter((p): p is { type: "text"; text: string } => p.type === "text")
          .map((p) => p.text)
          .join(""),
        timestamp: Date.now(),
      };
      setMessages((prev) => [...prev, myMessage]);
    },
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}
```

## Streaming Updates

```tsx
const runtime = useExternalStoreRuntime({
  messages,
  isRunning,
  onNew: async (message) => {
    addUserMessage(message);
    setRunning(true);

    // Create placeholder for assistant message
    const assistantId = crypto.randomUUID();
    addMessage({
      id: assistantId,
      role: "assistant",
      content: [{ type: "text", text: "" }],
      status: "running",
      createdAt: new Date(),
    });

    // Stream response
    const response = await fetch("/api/chat", {
      method: "POST",
      body: JSON.stringify({ messages }),
    });

    const reader = response.body?.getReader();
    const decoder = new TextDecoder();
    let fullText = "";

    while (reader) {
      const { done, value } = await reader.read();
      if (done) break;

      fullText += decoder.decode(value);

      // Update message in place
      updateMessage(assistantId, {
        content: [{ type: "text", text: fullText }],
        status: "running",
      });
    }

    // Mark complete
    updateMessage(assistantId, {
      content: [{ type: "text", text: fullText }],
      status: "complete",
    });
    setRunning(false);
  },
});
```

## With Edit and Reload

```tsx
const runtime = useExternalStoreRuntime({
  messages,
  isRunning,
  onNew: async (message) => {
    // Handle new message
  },
  onEdit: async (message) => {
    // Find message by parentId and create branch
    const parentIndex = messages.findIndex((m) => m.id === message.parentId);
    if (parentIndex === -1) return;

    // Replace messages after parent with edited message
    setMessages([
      ...messages.slice(0, parentIndex + 1),
      {
        id: crypto.randomUUID(),
        role: "user",
        content: message.content,
        status: "complete",
        createdAt: new Date(),
      },
    ]);

    // Regenerate response
    await generateResponse();
  },
  onReload: async (parentId) => {
    // Remove assistant message and regenerate
    const parentIndex = messages.findIndex((m) => m.id === parentId);
    setMessages(messages.slice(0, parentIndex + 1));
    await generateResponse();
  },
  onCancel: async () => {
    abortController.current?.abort();
    setRunning(false);
  },
});
```

## Capabilities

```tsx
const runtime = useExternalStoreRuntime({
  messages,
  isRunning,
  onNew: handleNew,
  // Only enable capabilities you implement
  capabilities: {
    edit: false,   // Disable edit if onEdit not provided
    reload: true,
    cancel: true,
    copy: true,
    speak: false,
    attachments: false,
  },
});
```
