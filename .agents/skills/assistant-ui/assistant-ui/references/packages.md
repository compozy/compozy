# assistant-ui Packages

## Published Packages

**To check latest version:** Run `npm view <package-name> version` or check the package on npmjs.com.

- Most published packages only expose the `latest` dist-tag; always install from `latest`.
- Monorepo-only: `@assistant-ui/x-buildutils` (not on npm).

| Package | Notes |
|---------|-------|
| @assistant-ui/react | Core UI library |
| @assistant-ui/react-ai-sdk | AI SDK v6 integration |
| @assistant-ui/react-langgraph | LangGraph integration |
| @assistant-ui/react-data-stream | Data stream utilities |
| @assistant-ui/react-markdown | Markdown rendering |
| @assistant-ui/react-syntax-highlighter | Code highlighting |
| @assistant-ui/store | State management |
| @assistant-ui/react-devtools | Developer tools |
| @assistant-ui/react-hook-form | React Hook Form integration |
| @assistant-ui/react-a2a | Agent-to-Agent protocol for multi-agent systems |
| @assistant-ui/react-ag-ui | AG-UI protocol adapter for agent backends |
| @assistant-ui/cloud-ai-sdk | AI SDK hooks for assistant-cloud persistence |
| @assistant-ui/core | Framework-agnostic core runtime |
| @assistant-ui/react-native | React Native bindings |
| @assistant-ui/react-o11y | Observability primitives |
| @assistant-ui/react-streamdown | Streamdown-based markdown rendering |
| @assistant-ui/tap | Reactive state management and testing |
| @assistant-ui/mcp-docs-server | MCP server for IDE integration |
| assistant-stream | Streaming protocol |
| assistant-cloud | Cloud persistence/auth |
| assistant-ui | CLI tool |
| create-assistant-ui | Project scaffolding |
| safe-content-frame | Sandboxed iframe content |
| tw-shimmer | Tailwind shimmer effects |
| tw-glass | Tailwind CSS v4 glass refraction effects |
| mcp-app-studio | MCP app builder |

## Core Packages

### @assistant-ui/react

Main UI library with primitives and hooks.

```bash
npm install @assistant-ui/react
```

**Exports:**
- Primitives: `ThreadPrimitive`, `MessagePrimitive`, `ComposerPrimitive`, `ActionBarPrimitive`, `BranchPickerPrimitive`, `AttachmentPrimitive`, `ThreadListPrimitive`
- Pre-built components are added via project templates in:
  `@/components/assistant-ui/thread` and `@/components/assistant-ui/thread-list`
- Hooks: `useAui`, `useAuiState`, `useAuiEvent`
- Runtime: `useLocalRuntime`, `useExternalStoreRuntime`
- Tools: `makeAssistantTool`, `makeAssistantToolUI`, `useAssistantTool`, `useAssistantToolUI`
- Provider: `AssistantRuntimeProvider`

### assistant-stream

Streaming protocol for AI responses.

```bash
npm install assistant-stream
```

**Exports:**
- `AssistantStream` - Core streaming abstraction
- `DataStreamEncoder/Decoder` - AI SDK format
- `AssistantTransportEncoder/Decoder` - Native format
- `PlainTextEncoder/Decoder` - Simple text streaming

### assistant-cloud

Cloud persistence and auth.

```bash
npm install assistant-cloud
```

**Exports:**
- `AssistantCloud` - Main client class
- Thread management, file uploads, auth

## Integration Packages

### @assistant-ui/react-ai-sdk

Vercel AI SDK v6 integration.

```bash
npm install @assistant-ui/react-ai-sdk @ai-sdk/react
```

**Exports:**
- `useChatRuntime` - Main hook (recommended)
- `useAISDKRuntime` - Lower-level hook
- `AssistantChatTransport` - Custom transport class

### @assistant-ui/react-langgraph

LangGraph agent integration.

```bash
npm install @assistant-ui/react-langgraph
```

**Exports:**
- `useLangGraphRuntime` - Main hook
- `useLangGraphSend`, `useLangGraphSendCommand` - Manual send control
- `useLangGraphInterruptState` - Interrupt state access
- `useLangGraphMessages` - Message state management
- `convertLangChainMessages`, `appendLangChainChunk` - Message converters
- `LangGraphMessageAccumulator` - Message accumulator

## UI Enhancement Packages

### @assistant-ui/react-markdown

Markdown rendering with syntax highlighting support.

```bash
npm install @assistant-ui/react-markdown
```

**Exports:**
- `MarkdownTextPrimitive` - Renders markdown content
- `useIsMarkdownCodeBlock` - Check if code block is inside markdown
- `unstable_memoizeMarkdownComponents` - Memoize markdown components for performance

### @assistant-ui/react-syntax-highlighter

Code block syntax highlighting.

```bash
npm install @assistant-ui/react-syntax-highlighter
```

## Package Selection Guide

| Scenario | Packages |
|----------|----------|
| Next.js + AI SDK | `@assistant-ui/react`, `@assistant-ui/react-ai-sdk`, `@ai-sdk/react` |
| LangGraph | `@assistant-ui/react`, `@assistant-ui/react-langgraph` |
| Custom backend | `@assistant-ui/react`, `assistant-stream` |
| With markdown | Add `@assistant-ui/react-markdown` |
| Production | Add `assistant-cloud` |

## Version Compatibility

- `@assistant-ui/react` requires React 18+ or 19
- `@assistant-ui/react-ai-sdk` requires AI SDK v6 (`ai@^6`)
- Node.js >=24 recommended (monorepo requirement)
