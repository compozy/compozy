# ActionBarPrimitive

Message action buttons (copy, edit, reload, etc.).

## Parts

| Part | Description |
|------|-------------|
| `.Root` | Container |
| `.Copy` | Copy message to clipboard |
| `.Edit` | Enter edit mode |
| `.Reload` | Regenerate response |
| `.Speak` | Text-to-speech |
| `.StopSpeaking` | Stop TTS |
| `.FeedbackPositive` | Thumbs up |
| `.FeedbackNegative` | Thumbs down |
| `.ExportMarkdown` | Export message |

## Basic Usage

```tsx
<ActionBarPrimitive.Root>
  <ActionBarPrimitive.Copy />
  <ActionBarPrimitive.Reload />
  <ActionBarPrimitive.Edit />
</ActionBarPrimitive.Root>
```

## ActionBarPrimitive.Root

Container for action buttons. Usually placed inside a message.

```tsx
<ActionBarPrimitive.Root
  className="flex gap-2 mt-2"
  hideWhenRunning  // Hide while generating
  autohide="not-last"  // "always" | "not-last" | "never"
  autohideFloat="single-branch"  // Float behavior
>
  {children}
</ActionBarPrimitive.Root>
```

### Props

- `hideWhenRunning` - Hide while assistant is generating
- `autohide` - Only show on message hover
- `autohideFloat` - Float positioning mode

## ActionBarPrimitive.Copy

Copy message content to clipboard.

```tsx
<ActionBarPrimitive.Copy
  className="p-1 rounded hover:bg-gray-100"
  copiedDuration={2000}  // Duration of "copied" state
>
  {/* Default content */}
  <CopyIcon className="w-4 h-4" />
</ActionBarPrimitive.Copy>

// With copied state
<ActionBarPrimitive.Copy>
  <AuiIf condition={({ message }) => message.isCopied}>
    <CheckIcon className="w-4 h-4 text-green-500" />
  </AuiIf>
  <AuiIf condition={({ message }) => !message.isCopied}>
    <CopyIcon className="w-4 h-4" />
  </AuiIf>
</ActionBarPrimitive.Copy>
```

## ActionBarPrimitive.Reload

Regenerate the assistant's response.

```tsx
<ActionBarPrimitive.Reload className="p-1 rounded hover:bg-gray-100">
  <RefreshIcon className="w-4 h-4" />
  Regenerate
</ActionBarPrimitive.Reload>
```

## ActionBarPrimitive.Edit

Enter edit mode for user messages.

```tsx
<AuiIf condition={({ message }) => message.role === "user"}>
  <ActionBarPrimitive.Edit className="p-1 rounded hover:bg-gray-100">
    <EditIcon className="w-4 h-4" />
    Edit
  </ActionBarPrimitive.Edit>
</AuiIf>
```

## ActionBarPrimitive.Speak / StopSpeaking

Text-to-speech controls.

```tsx
<AuiIf condition={({ message }) => message.speech == null}>
  <ActionBarPrimitive.Speak className="p-1 rounded hover:bg-gray-100">
    🔊 Read aloud
  </ActionBarPrimitive.Speak>
</AuiIf>

<AuiIf condition={({ message }) => message.speech != null}>
  <ActionBarPrimitive.StopSpeaking className="p-1 rounded bg-red-100">
    ⏹️ Stop
  </ActionBarPrimitive.StopSpeaking>
</AuiIf>
```

## ActionBarPrimitive.FeedbackPositive / FeedbackNegative

Thumbs up/down feedback buttons.

```tsx
<ActionBarPrimitive.FeedbackPositive
  className="p-1 rounded hover:bg-gray-100"
>
  👍
</ActionBarPrimitive.FeedbackPositive>

<ActionBarPrimitive.FeedbackNegative>
  👎
</ActionBarPrimitive.FeedbackNegative>
```

Requires a feedback adapter in the runtime:

```tsx
const runtime = useChatRuntime({
  transport: new AssistantChatTransport({
    api: "/api/chat",
  }),
  adapters: {
    feedback: {
      submit: async ({ messageId, type }) => {
        await fetch("/api/feedback", {
          method: "POST",
          body: JSON.stringify({ messageId, type }),
        });
      },
    },
  },
});
```

Use `AuiIf` for copy/speech conditional rendering instead of `ActionBarPrimitive.If`.

## Complete Example

```tsx
function MessageActionBar() {
  return (
    <ActionBarPrimitive.Root
      className="flex items-center gap-1 opacity-0 group-hover:opacity-100 transition-opacity"
      hideWhenRunning
    >
      {/* Copy */}
      <ActionBarPrimitive.Copy
        className="p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100"
        copiedDuration={2000}
      >
        <AuiIf condition={({ message }) => message.isCopied}>
          <CheckIcon className="w-4 h-4 text-green-500" />
        </AuiIf>
        <AuiIf condition={({ message }) => !message.isCopied}>
          <CopyIcon className="w-4 h-4" />
        </AuiIf>
      </ActionBarPrimitive.Copy>

      {/* Regenerate (assistant only) */}
      <AuiIf condition={({ message }) => message.role === "assistant"}>
        <ActionBarPrimitive.Reload className="p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100">
          <RefreshIcon className="w-4 h-4" />
        </ActionBarPrimitive.Reload>
      </AuiIf>

      {/* Edit (user only) */}
      <AuiIf condition={({ message }) => message.role === "user"}>
        <ActionBarPrimitive.Edit className="p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100">
          <EditIcon className="w-4 h-4" />
        </ActionBarPrimitive.Edit>
      </AuiIf>

      {/* Text-to-speech */}
      <AuiIf condition={({ message }) => message.speech == null}>
        <ActionBarPrimitive.Speak className="p-1.5 rounded text-gray-500 hover:text-gray-700 hover:bg-gray-100">
          <SpeakerIcon className="w-4 h-4" />
        </ActionBarPrimitive.Speak>
      </AuiIf>
      <AuiIf condition={({ message }) => message.speech != null}>
        <ActionBarPrimitive.StopSpeaking className="p-1.5 rounded text-red-500 bg-red-50">
          <StopIcon className="w-4 h-4" />
        </ActionBarPrimitive.StopSpeaking>
      </AuiIf>

      {/* Feedback */}
      <div className="border-l pl-1 ml-1">
        <ActionBarPrimitive.FeedbackPositive
          className="p-1.5 rounded text-gray-500 hover:text-green-600 hover:bg-green-50"
        >
          <ThumbsUpIcon className="w-4 h-4" />
        </ActionBarPrimitive.FeedbackPositive>
        <ActionBarPrimitive.FeedbackNegative
          className="p-1.5 rounded text-gray-500 hover:text-red-600 hover:bg-red-50"
        >
          <ThumbsDownIcon className="w-4 h-4" />
        </ActionBarPrimitive.FeedbackNegative>
      </div>
    </ActionBarPrimitive.Root>
  );
}
```

## Using with Messages

```tsx
function AssistantMessage() {
  return (
    <MessagePrimitive.Root className="group flex mb-4">
      <Avatar fallback="AI" />
      <div className="flex-1">
        <MessagePrimitive.Content />
        <MessageActionBar />
      </div>
    </MessagePrimitive.Root>
  );
}
```

Note the `group` class on `.Root` to enable hover state propagation.
