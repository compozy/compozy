# ComposerPrimitive

Message input form for sending messages.

## Parts

| Part | Description |
|------|-------------|
| `.Root` | Form container |
| `.Input` | Text input/textarea |
| `.Send` | Submit button |
| `.Cancel` | Cancel generation |
| `.AddAttachment` | Attach files button |
| `.Attachments` | Render attachments |
| `.AttachmentDropzone` | Drag-drop area |
| `.Dictate` | Start voice input |
| `.StopDictation` | Stop voice input |
| `.If` | Conditional rendering (deprecated; prefer `AuiIf`) |

## Basic Structure

```tsx
<ComposerPrimitive.Root>
  <ComposerPrimitive.Input placeholder="Type a message..." />
  <ComposerPrimitive.Send>Send</ComposerPrimitive.Send>
</ComposerPrimitive.Root>
```

## ComposerPrimitive.Root

Form element that handles submission.

```tsx
<ComposerPrimitive.Root
  className="flex gap-2 p-4 border-t"
  onSubmit={() => console.log("Submitted")}
>
  {children}
</ComposerPrimitive.Root>
```

## ComposerPrimitive.Input

Auto-resizing textarea for message input.

```tsx
<ComposerPrimitive.Input
  className="flex-1 resize-none rounded-lg border px-4 py-2"
  placeholder="Type a message..."
  rows={1}
  autoFocus
/>
```

### Props

- `placeholder` - Placeholder text
- `rows` - Initial row count (auto-resizes)
- `autoFocus` - Focus on mount
- `disabled` - Disable input
- Standard textarea props

## ComposerPrimitive.Send

Submit button. Disabled when input is empty or generating.

```tsx
<ComposerPrimitive.Send
  className="bg-blue-500 text-white px-4 py-2 rounded-lg disabled:opacity-50"
>
  Send
</ComposerPrimitive.Send>
```

## ComposerPrimitive.Cancel

Cancel ongoing generation.

```tsx
<AuiIf condition={({ thread }) => thread.isRunning}>
  <ComposerPrimitive.Cancel className="bg-red-500 text-white px-4 py-2 rounded-lg">
    Stop
  </ComposerPrimitive.Cancel>
</AuiIf>
```

## Conditional rendering with `AuiIf`

Deprecated `ComposerPrimitive.If` supports only `editing` and `dictation` props.
Prefer `AuiIf` for richer state checks (`thread`, `composer`, etc.).

```tsx
// While sending
<AuiIf condition={({ thread }) => thread.isRunning}>
  <ComposerPrimitive.Cancel>Stop</ComposerPrimitive.Cancel>
</AuiIf>

// Not generating
<AuiIf condition={({ thread }) => !thread.isRunning}>
  <ComposerPrimitive.Send>Send</ComposerPrimitive.Send>
</AuiIf>

// Has file attachments
<AuiIf condition={({ composer }) => composer.attachments.length > 0}>
  <AttachmentList />
</AuiIf>
```

### Available Conditions

- `thread.isRunning` - Thread is currently generating
- `composer.attachments.length > 0` - Composer has file attachments
- `composer.isEditing` - Composer is in edit mode
- `composer.dictation != null` - Dictation is active

## Attachments

### Add Attachment Button

```tsx
<ComposerPrimitive.AddAttachment
  className="p-2 rounded hover:bg-gray-100"
>
  📎 Attach
</ComposerPrimitive.AddAttachment>
```

### Attachment List

```tsx
<ComposerPrimitive.Attachments className="flex gap-2 mb-2">
  <AttachmentPrimitive.Root className="flex items-center gap-1 bg-gray-100 rounded px-2 py-1">
    <AttachmentPrimitive.Name className="text-sm" />
    <AttachmentPrimitive.Remove className="text-red-500">×</AttachmentPrimitive.Remove>
  </AttachmentPrimitive.Root>
</ComposerPrimitive.Attachments>
```

### Drag-Drop Zone

```tsx
<ComposerPrimitive.AttachmentDropzone
  className="border-2 border-dashed rounded-lg p-4 text-center"
>
  Drop files here
</ComposerPrimitive.AttachmentDropzone>
```

## Voice Input

```tsx
<AuiIf condition={({ composer }) => composer.dictation == null}>
  <ComposerPrimitive.Dictate className="p-2 rounded hover:bg-gray-100">
    🎤 Voice
  </ComposerPrimitive.Dictate>
</AuiIf>

<AuiIf condition={({ composer }) => composer.dictation != null}>
  <ComposerPrimitive.StopDictation className="p-2 rounded bg-red-100">
    ⏹️ Stop
  </ComposerPrimitive.StopDictation>
</AuiIf>
```

## Complete Example

```tsx
function CustomComposer() {
  return (
    <ComposerPrimitive.Root className="border-t p-4">
      {/* Drag-drop overlay */}
      <ComposerPrimitive.AttachmentDropzone className="absolute inset-0 flex items-center justify-center bg-blue-50/80 border-2 border-dashed border-blue-300 rounded-lg opacity-0 pointer-events-none data-dragging:opacity-100 data-dragging:pointer-events-auto">
        <p className="text-blue-500">Drop files to attach</p>
      </ComposerPrimitive.AttachmentDropzone>

      {/* Attached files */}
      <AuiIf condition={({ composer }) => composer.attachments.length > 0}>
        <ComposerPrimitive.Attachments className="flex flex-wrap gap-2 mb-2">
          <AttachmentPrimitive.Root className="group flex items-center gap-2 bg-gray-100 rounded-lg px-3 py-1.5">
            <AttachmentPrimitive.Name className="text-sm truncate max-w-[150px]" />
            <AttachmentPrimitive.Remove className="text-gray-400 hover:text-red-500 opacity-0 group-hover:opacity-100">
              ×
            </AttachmentPrimitive.Remove>
          </AttachmentPrimitive.Root>
        </ComposerPrimitive.Attachments>
      </AuiIf>

      {/* Input row */}
      <div className="flex items-end gap-2">
        <ComposerPrimitive.AddAttachment className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded">
          <PaperclipIcon className="w-5 h-5" />
        </ComposerPrimitive.AddAttachment>

        <ComposerPrimitive.Input
          className="flex-1 max-h-40 resize-none rounded-lg border border-gray-200 px-4 py-2 focus:outline-none focus:ring-2 focus:ring-blue-500"
          placeholder="Type a message..."
          rows={1}
        />

        <AuiIf condition={({ composer }) => composer.dictation == null}>
          <ComposerPrimitive.Dictate className="p-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 rounded">
            <MicIcon className="w-5 h-5" />
          </ComposerPrimitive.Dictate>
        </AuiIf>

        <AuiIf condition={({ composer }) => composer.dictation != null}>
          <ComposerPrimitive.StopDictation className="p-2 text-red-500 bg-red-50 rounded animate-pulse">
            <StopIcon className="w-5 h-5" />
          </ComposerPrimitive.StopDictation>
        </AuiIf>

        <AuiIf condition={({ thread }) => thread.isRunning}>
          <ComposerPrimitive.Cancel className="p-2 text-red-500 hover:bg-red-50 rounded">
            <StopIcon className="w-5 h-5" />
          </ComposerPrimitive.Cancel>
        </AuiIf>

        <AuiIf condition={({ thread }) => !thread.isRunning}>
          <ComposerPrimitive.Send className="p-2 bg-blue-500 text-white rounded hover:bg-blue-600 disabled:opacity-50 disabled:hover:bg-blue-500">
            <SendIcon className="w-5 h-5" />
          </ComposerPrimitive.Send>
        </AuiIf>
      </div>
    </ComposerPrimitive.Root>
  );
}
```

## Accessing Composer State

```tsx
import { useComposer, useComposerRuntime } from "@assistant-ui/react";

function ComposerInfo() {
  // Reactive state
  const { text, attachments, isSubmitting } = useComposer();

  // Runtime API
  const runtime = useComposerRuntime();
  const handleClear = () => runtime.setText("");

  return (
    <div>
      <p>Characters: {text.length}</p>
      <p>Attachments: {attachments.length}</p>
      <button onClick={handleClear}>Clear</button>
    </div>
  );
}
```
