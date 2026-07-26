# ThreadPrimitive

Container for the entire chat thread.

## Parts

| Part | Description |
|------|-------------|
| `.Root` | Outermost container element |
| `.Viewport` | Scrollable message area |
| `.Messages` | Renders message list |
| `.Empty` | Shown when no messages |
| `.ScrollToBottom` | Button to scroll down |
| `.Suggestions` | Quick reply suggestions |
| `.If` | Conditional rendering (deprecated; prefer `AuiIf`) |

## Basic Structure

```tsx
<ThreadPrimitive.Root>
  <ThreadPrimitive.Viewport>
    <ThreadPrimitive.Messages
      components={{
        UserMessage: MyUserMessage,
        AssistantMessage: MyAssistantMessage,
      }}
    />
  </ThreadPrimitive.Viewport>
</ThreadPrimitive.Root>
```

## ThreadPrimitive.Root

Container element. Accepts standard div props.

```tsx
<ThreadPrimitive.Root
  className="flex flex-col h-full"
  data-testid="chat-thread"
>
  {children}
</ThreadPrimitive.Root>
```

## ThreadPrimitive.Viewport

Scrollable area containing messages. Handles auto-scroll on new messages.

```tsx
<ThreadPrimitive.Viewport
  className="flex-1 overflow-y-auto p-4"
  autoScroll={true}  // Default: true
>
  <ThreadPrimitive.Messages />
</ThreadPrimitive.Viewport>
```

## ThreadPrimitive.Messages

Renders the message list.

```tsx
<ThreadPrimitive.Messages
  components={{
    // Required: Message components
    UserMessage: () => <MessagePrimitive.Root>...</MessagePrimitive.Root>,
    AssistantMessage: () => <MessagePrimitive.Root>...</MessagePrimitive.Root>,

    // Optional: System message
    SystemMessage: ({ message }) => <div>{message.content}</div>,

    // Optional: Edit composer for message editing
    EditComposer: () => <ComposerPrimitive.Root>...</ComposerPrimitive.Root>,
  }}
/>
```

## ThreadPrimitive.Empty

Rendered when thread has no messages.

```tsx
<ThreadPrimitive.Empty className="flex-1 flex items-center justify-center">
  <div className="text-center">
    <h2>Welcome!</h2>
    <p>Start a conversation</p>
  </div>
</ThreadPrimitive.Empty>
```

## ThreadPrimitive.ScrollToBottom

Button that appears when scrolled up, scrolls to bottom on click.

```tsx
<ThreadPrimitive.ScrollToBottom
  className="fixed bottom-20 right-4 rounded-full p-2 bg-white shadow"
>
  ↓ Scroll to bottom
</ThreadPrimitive.ScrollToBottom>
```

## ThreadPrimitive.Suggestions

Renders suggested quick replies.

```tsx
<ThreadPrimitive.Suggestions
  components={{
    Suggestion: ({ suggestion }) => (
      <button onClick={() => suggestion.onClick()}>
        {suggestion.text}
      </button>
    ),
  }}
/>
```

## Conditional Rendering (`AuiIf`)

`ThreadPrimitive.If` is deprecated. Prefer `AuiIf` with thread state:

```tsx
// When no messages
<AuiIf condition={({ thread }) => thread.isEmpty}>
  <EmptyState />
</AuiIf>

// When has messages
<AuiIf condition={({ thread }) => !thread.isEmpty}>
  <ThreadPrimitive.Messages />
</AuiIf>

// While generating
<AuiIf condition={({ thread }) => thread.isRunning}>
  <LoadingIndicator />
</AuiIf>
```

### Available Conditions

- `thread.isEmpty` - Thread has no messages
- `thread.isRunning` - Generation in progress
- `thread.isDisabled` - Thread is disabled

## Complete Example

```tsx
function CustomThread() {
  return (
    <ThreadPrimitive.Root className="relative flex flex-col h-full bg-white">
      {/* Empty state */}
      <AuiIf condition={({ thread }) => thread.isEmpty}>
        <div className="flex-1 flex items-center justify-center">
          <div className="text-center p-8">
            <h1 className="text-2xl font-bold mb-2">AI Assistant</h1>
            <p className="text-gray-500 mb-4">How can I help you today?</p>
            <ThreadPrimitive.Suggestions
              components={{
                Suggestion: ({ suggestion }) => (
                  <button
                    className="m-1 px-4 py-2 bg-gray-100 rounded-full"
                    onClick={suggestion.onClick}
                  >
                    {suggestion.text}
                  </button>
                ),
              }}
            />
          </div>
        </div>
      </AuiIf>

      {/* Messages */}
      <AuiIf condition={({ thread }) => !thread.isEmpty}>
        <ThreadPrimitive.Viewport className="flex-1 overflow-y-auto">
          <div className="max-w-3xl mx-auto p-4">
            <ThreadPrimitive.Messages
              components={{
                UserMessage: CustomUserMessage,
                AssistantMessage: CustomAssistantMessage,
              }}
            />
          </div>
        </ThreadPrimitive.Viewport>
      </AuiIf>

      {/* Scroll to bottom */}
      <ThreadPrimitive.ScrollToBottom className="absolute bottom-24 right-4 p-2 rounded-full bg-white shadow-lg hover:bg-gray-50">
        <ChevronDownIcon className="w-5 h-5" />
      </ThreadPrimitive.ScrollToBottom>

      {/* Composer */}
      <div className="border-t bg-white">
        <CustomComposer />
      </div>
    </ThreadPrimitive.Root>
  );
}
```

## Accessing Thread State

```tsx
import { useThread, useThreadRuntime } from "@assistant-ui/react";

function ThreadInfo() {
  // Reactive state
  const { messages, isRunning } = useThread();

  // Runtime API
  const runtime = useThreadRuntime();
  const handleClear = () => runtime.startRun();

  return (
    <div>
      <p>{messages.length} messages</p>
      {isRunning && <p>Generating...</p>}
    </div>
  );
}
```
