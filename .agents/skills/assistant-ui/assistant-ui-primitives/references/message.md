# MessagePrimitive

Individual message display.

## Parts

| Part | Description |
|------|-------------|
| `.Root` | Message container |
| `.Parts` | Message body with parts (canonical) |
| `.Content` | Message body with parts |
| `.If` | Conditional rendering (deprecated; prefer `AuiIf`) |
| `.Error` | Render fallback when message has an error |
| `.PartByIndex` | Render a single part by index |
| `.Attachments` | Render message attachments |
| `.AttachmentByIndex` | Render one attachment by index |

## Basic Structure

```tsx
<MessagePrimitive.Root>
  <Avatar src="/user-avatar.png" />
  <MessagePrimitive.Content />
</MessagePrimitive.Root>
```

## MessagePrimitive.Root

Container for a single message.

```tsx
<MessagePrimitive.Root
  className="flex gap-2 mb-4"
  data-role="user"  // or "assistant"
>
  {children}
</MessagePrimitive.Root>
```

## MessagePrimitive.Content

Renders message content parts (text, images, tool calls, etc.).

```tsx
// Simple usage - uses default rendering
<MessagePrimitive.Content />

// Custom part rendering
<MessagePrimitive.Content
  components={{
    Text: ({ part }) => (
      <p className="whitespace-pre-wrap">{part.text}</p>
    ),
    Image: ({ part }) => (
      <img src={part.image} alt="" className="max-w-full rounded" />
    ),
    ToolCall: ({ part }) => (
      <div className="bg-gray-100 rounded p-2">
        <strong>{part.toolName}</strong>
        {part.result && <pre>{JSON.stringify(part.result, null, 2)}</pre>}
      </div>
    ),
    Reasoning: ({ part }) => (
      <details className="text-gray-500">
        <summary>Thinking...</summary>
        <p>{part.text}</p>
      </details>
    ),
    Source: ({ part }) => (
      <a href={part.url} className="text-blue-500">
        {part.title}
      </a>
    ),
    File: ({ part }) => (
      <a
        href={`data:${part.mimeType};base64,${part.data}`}
        download={part.filename ?? "file"}
      >
        📄 {part.filename ?? "file"}
      </a>
    ),
  }}
/>
```

### Part Types

| Type | Description | Properties |
|------|-------------|------------|
| `Text` | Plain text | `text` |
| `Image` | Image attachment | `image` (URL) |
| `ToolCall` | Tool invocation | `toolName`, `args`, `argsText`, `result?`, `isError?`, `artifact?` |
| `Reasoning` | Chain-of-thought | `text` |
| `Source` | Citation/reference | `url`, `title` |
| `File` | File attachment | `filename?`, `data`, `mimeType` |

## MessagePrimitive.If / AuiIf

`MessagePrimitive.If` still exists but is deprecated. Prefer `AuiIf` for the most flexible state checks.

```tsx
<MessagePrimitive.If user>User message content</MessagePrimitive.If>
<MessagePrimitive.If assistant>Assistant message content</MessagePrimitive.If>
<MessagePrimitive.If system>System message content</MessagePrimitive.If>
<MessagePrimitive.If hasBranches>
  <BranchPickerPrimitive.Root>...</BranchPickerPrimitive.Root>
</MessagePrimitive.If>

<MessagePrimitive.If copied>Copied</MessagePrimitive.If>
<MessagePrimitive.If speaking>Playing speech</MessagePrimitive.If>
<MessagePrimitive.If submittedFeedback="positive">Positive feedback</MessagePrimitive.If>
```

When you need custom conditions (for example branch metadata), use `AuiIf`:

```tsx
<AuiIf condition={({ message }) => message.branchCount > 1}>
  <BranchPickerPrimitive.Root>...</BranchPickerPrimitive.Root>
</AuiIf>

<AuiIf condition={({ message }) => message.isCopied}>
  <CheckIcon />
</AuiIf>
```

## Complete Example

```tsx
function CustomUserMessage() {
  return (
    <MessagePrimitive.Root className="flex justify-end mb-4">
      <Avatar
        fallback="U"
        className="w-8 h-8 rounded-full bg-blue-500 text-white flex items-center justify-center ml-2"
      />
      <div className="max-w-[80%]">
        <div className="bg-blue-500 text-white rounded-2xl rounded-tr-sm px-4 py-2">
          <MessagePrimitive.Content />
        </div>
      </div>
    </MessagePrimitive.Root>
  );
}

function CustomAssistantMessage() {
  return (
    <MessagePrimitive.Root className="flex mb-4">
      <Avatar
        src="/ai-avatar.png"
        fallback="AI"
        className="w-8 h-8 rounded-full mr-2 shrink-0"
      />

      <div className="max-w-[80%]">
        <div className="bg-gray-100 rounded-2xl rounded-tl-sm px-4 py-2">
          <MessagePrimitive.Content />
        </div>
      </div>

      <ActionBarPrimitive.Root className="flex gap-2 mt-1 opacity-0 hover:opacity-100 transition-opacity">
        <ActionBarPrimitive.Copy className="text-xs text-gray-500 hover:text-gray-700">
          Copy
        </ActionBarPrimitive.Copy>
        <ActionBarPrimitive.Reload className="text-xs text-gray-500 hover:text-gray-700">
          Regenerate
        </ActionBarPrimitive.Reload>
        <ActionBarPrimitive.Speak className="text-xs text-gray-500 hover:text-gray-700">
          🔊
        </ActionBarPrimitive.Speak>
      </ActionBarPrimitive.Root>
    </MessagePrimitive.Root>
  );
}
```

## Error and branching support

Use `MessagePrimitive.Error` to render a fallback UI only when the message has an error:

```tsx
<MessagePrimitive.Error>
  <ErrorPrimitive.Root>
    <ErrorPrimitive.Message />
  </ErrorPrimitive.Root>
</MessagePrimitive.Error>
```

## Accessing Message State

```tsx
import { useMessage, useMessageRuntime } from "@assistant-ui/react";

function MessageInfo() {
  // Reactive state
  const { role, content, status, createdAt } = useMessage();

  // Runtime API
  const runtime = useMessageRuntime();
  const handleEdit = () => runtime.edit({
    role: "user",
    content: [{ type: "text", text: "New content" }],
  });

  return (
    <div>
      <p>Role: {role}</p>
      <p>Status: {status}</p>
    </div>
  );
}
```
