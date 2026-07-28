# Encoders and Decoders

Encode and decode streaming formats.

## Available Encoders

| Encoder | Format | Use Case |
|---------|--------|----------|
| `DataStreamEncoder` | AI SDK Data Stream | Default (used by `toUIMessageStreamResponse`) |
| `AssistantTransportEncoder` | Native SSE (`data: {chunk}`) | Custom backends that want all chunk types |
| `PlainTextEncoder` | Text-only | Very simple demos |

## DataStreamEncoder

AI SDK compatible format. You normally don't call it directly—wrap an `AssistantStream`:

```ts
import { AssistantStream, DataStreamEncoder, DataStreamDecoder } from "assistant-stream";

// Server
const response = AssistantStream.toResponse(stream, new DataStreamEncoder());

// Client
const stream = AssistantStream.fromResponse(response, new DataStreamDecoder());
for await (const chunk of stream) {
  console.log(chunk);
}
```

## AssistantTransportEncoder

Native assistant-ui format with all features.

```ts
import {
  AssistantTransportEncoder,
  AssistantTransportDecoder,
} from "assistant-stream";

// Encoding: wrap AssistantStream chunks
const response = AssistantStream.toResponse(stream, new AssistantTransportEncoder());

// Decoding
const stream = AssistantStream.fromResponse(response, new AssistantTransportDecoder());
for await (const chunk of stream) {
  console.log(chunk);
}
```

## PlainTextEncoder

Simple text-only streaming.

```ts
import { PlainTextEncoder, PlainTextDecoder } from "assistant-stream";

// Encoding
const encoder = new PlainTextEncoder();
const stream = encoder.encode("Hello world!");

// Decoding
const decoder = new PlainTextDecoder();
for await (const text of decoder.decode(stream)) {
  console.log(text);
}
```

## UIMessageStreamDecoder

Optimized for UI rendering - accumulates into message state.

```ts
import { UIMessageStreamDecoder } from "assistant-stream";

const decoder = new UIMessageStreamDecoder();

for await (const update of decoder.decode(stream)) {
  // update contains full message state ready for UI
  setMessages(update.messages);
}
```

## Creating Custom Streams

### From Response

```ts
const response = await fetch("/api/chat", { ... });
const stream = AssistantStream.fromResponse(response, new DataStreamDecoder());
```

## Server Response Helpers

### Create Streaming Response

Use `createAssistantStreamController` to build an `AssistantStream` and encode it:

```ts
import {
  AssistantStream,
  AssistantTransportEncoder,
  createAssistantStreamController,
} from "assistant-stream";

export async function POST() {
  const [stream, controller] = createAssistantStreamController();

  controller.appendText("Hello ");
  controller.appendText("world!");
  controller.close();

  return AssistantStream.toResponse(stream, new AssistantTransportEncoder());
}
```

## Debugging

### Log Raw Stream

```ts
const response = await fetch("/api/chat", { ... });
const reader = response.body?.getReader();
const decoder = new TextDecoder();

while (reader) {
  const { done, value } = await reader.read();
  if (done) break;
  console.log("Raw:", decoder.decode(value));
}
```

### Validate Format

```ts
// Check if response is valid SSE
const contentType = response.headers.get("Content-Type");
console.log("Content-Type:", contentType);  // Should be text/event-stream
```
