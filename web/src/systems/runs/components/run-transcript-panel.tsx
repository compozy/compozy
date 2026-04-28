import { useCallback, useMemo, useState, type ReactElement, type ReactNode } from "react";

import {
  AssistantRuntimeProvider,
  DataRenderers,
  MessagePrimitive,
  ThreadPrimitive,
  Tools,
  makeAssistantDataUI,
  useAui,
  useExternalStoreRuntime,
  type ThreadMessageLike,
  type ToolCallMessagePartProps,
  type Toolkit,
} from "@assistant-ui/react";
import {
  Activity,
  AlertCircle,
  Brain,
  ChevronDown,
  ChevronRight,
  Code2,
  FileText,
  Loader2,
  Terminal,
  Wrench,
} from "lucide-react";

import {
  Alert,
  EmptyState,
  Markdown,
  StatusBadge,
  SurfaceCard,
  SurfaceCardBody,
  SurfaceCardDescription,
  SurfaceCardEyebrow,
  SurfaceCardHeader,
  SurfaceCardTitle,
  cn,
} from "@compozy/ui";

import type { RunFeedEvent } from "../lib/event-store";
import type { RunTranscript, RunUIMessage, RunUIMessagePart } from "../types";

type ThreadPart = Exclude<ThreadMessageLike["content"], string>[number];
type JSONValue = string | number | boolean | null | JSONValue[] | { [key: string]: JSONValue };
type JSONObject = { [key: string]: JSONValue };

interface RunTranscriptPanelProps {
  transcript?: RunTranscript;
  liveEvents?: readonly RunFeedEvent[];
  isLoading?: boolean;
  isError?: boolean;
  errorMessage?: string | null;
  compact?: boolean;
  testId?: string;
  title?: string;
  description?: string;
}

interface CompozyBlockData {
  entry_kind?: string;
  entry_id?: string;
  block?: unknown;
}

interface CompozyEventData {
  type?: string;
  title?: string;
  text?: string;
  blocks?: unknown[];
  tool_call_id?: string;
  tool_call_state?: string;
}

const registeredToolNames = [
  "Bash",
  "Read",
  "Write",
  "Edit",
  "Grep",
  "Glob",
  "WebSearch",
  "WebFetch",
  "Task",
  "Agent",
  "Think",
  "TodoWrite",
  "tool",
] as const;

const runToolkit = registeredToolNames.reduce<Toolkit>((toolkit, toolName) => {
  toolkit[toolName] = {
    type: "backend",
    render: part => <RunToolCard part={part} />,
  };
  return toolkit;
}, {});

export function RunTranscriptPanel({
  transcript,
  liveEvents = [],
  isLoading = false,
  isError = false,
  errorMessage = null,
  compact = false,
  testId = "run-detail-transcript",
  title,
  description = "Structured ACP messages, reasoning, tools, and runtime notices.",
}: RunTranscriptPanelProps): ReactElement {
  const messages = useMemo(
    () => mergeTranscriptMessages(transcript?.messages ?? [], liveEvents),
    [liveEvents, transcript?.messages]
  );
  const resolvedTitle = title ?? (compact ? "Run log" : "Assistant log");

  return (
    <SurfaceCard data-testid={testId}>
      <SurfaceCardHeader>
        <div>
          <SurfaceCardEyebrow>Transcript</SurfaceCardEyebrow>
          <SurfaceCardTitle>{resolvedTitle}</SurfaceCardTitle>
          <SurfaceCardDescription>{description}</SurfaceCardDescription>
        </div>
        <StatusBadge tone={transcript?.incomplete ? "warning" : "info"}>
          {messages.length}
        </StatusBadge>
      </SurfaceCardHeader>
      <SurfaceCardBody>
        {isError ? (
          <Alert data-testid={`${testId}-error`} variant="error">
            {errorMessage ?? "Failed to load run transcript"}
          </Alert>
        ) : null}
        {isLoading && messages.length === 0 ? (
          <div className="space-y-2" data-testid={`${testId}-loading`}>
            <div className="h-16 rounded-[var(--radius-md)] bg-muted" />
            <div className="h-24 rounded-[var(--radius-md)] bg-muted" />
          </div>
        ) : null}
        {!isLoading && messages.length === 0 && !isError ? (
          <EmptyState
            data-testid={`${testId}-empty`}
            description="Structured ACP transcript entries will appear after the agent writes output."
            title="Transcript is empty"
          />
        ) : null}
        {messages.length > 0 ? (
          <RunTranscriptRuntimeProvider messages={messages}>
            <RunThread compact={compact} />
          </RunTranscriptRuntimeProvider>
        ) : null}
        {transcript?.incomplete ? (
          <Alert className="mt-4" data-testid={`${testId}-incomplete`} variant="warning">
            Transcript may be incomplete: {(transcript.incomplete_reasons ?? []).join(", ")}
          </Alert>
        ) : null}
      </SurfaceCardBody>
    </SurfaceCard>
  );
}

function RunTranscriptRuntimeProvider({
  children,
  messages,
}: {
  children: ReactNode;
  messages: readonly RunUIMessage[];
}): ReactElement {
  const convertMessage = useCallback((message: RunUIMessage): ThreadMessageLike => {
    return {
      id: message.id,
      role: message.role,
      content: message.parts.flatMap(part => convertRunMessagePart(part)),
      status: { type: "complete", reason: "stop" },
    };
  }, []);

  const runtime = useExternalStoreRuntime<RunUIMessage>({
    messages,
    convertMessage,
    isDisabled: true,
    onNew: async () => {
      throw new Error("Run transcripts are read-only");
    },
    unstable_capabilities: { copy: true },
  });
  const aui = useAui({
    tools: Tools({ toolkit: runToolkit }),
    dataRenderers: DataRenderers(),
  });
  const EventDataUI = useMemo(
    () =>
      makeAssistantDataUI<CompozyEventData>({
        name: "compozy-event",
        render: ({ data }) => <RunEventPart data={data} />,
      }),
    []
  );
  const BlockDataUI = useMemo(
    () =>
      makeAssistantDataUI<CompozyBlockData>({
        name: "compozy-block",
        render: ({ data }) => <RunBlockPart data={data} />,
      }),
    []
  );

  return (
    <AssistantRuntimeProvider aui={aui} runtime={runtime}>
      <EventDataUI />
      <BlockDataUI />
      {children}
    </AssistantRuntimeProvider>
  );
}

function RunThread({ compact }: { compact: boolean }): ReactElement {
  return (
    <ThreadPrimitive.Root className="min-w-0">
      <ThreadPrimitive.Viewport
        className={cn(
          "max-h-[min(70dvh,760px)] min-h-0 overflow-y-auto pr-1",
          compact ? "space-y-2" : "space-y-3"
        )}
        data-testid="run-transcript-thread"
      >
        <ThreadPrimitive.Messages
          components={{
            AssistantMessage: () => <AssistantMessage compact={compact} />,
            UserMessage,
          }}
        />
      </ThreadPrimitive.Viewport>
    </ThreadPrimitive.Root>
  );
}

function UserMessage(): ReactElement {
  return (
    <MessagePrimitive.Root className="flex justify-end py-2">
      <div className="max-w-[min(82%,42rem)] rounded-[var(--radius-md)] border border-border-subtle bg-muted px-3 py-2">
        <MessagePrimitive.Parts
          components={{
            Text: ({ text }) => <RunTextPart text={text} />,
          }}
        />
      </div>
    </MessagePrimitive.Root>
  );
}

function AssistantMessage({ compact }: { compact: boolean }): ReactElement {
  return (
    <MessagePrimitive.Root
      className={cn(
        "grid grid-cols-[1.25rem_minmax(0,1fr)] gap-3 border-l border-border-subtle py-3 pl-3",
        compact ? "py-2" : "py-3"
      )}
    >
      <div className="mt-1 flex size-5 items-center justify-center rounded-full border border-border-subtle bg-[color:var(--surface-inset)] text-muted-foreground">
        <Activity className="size-3" />
      </div>
      <div className="min-w-0 space-y-3">
        <MessagePrimitive.Parts
          components={{
            Text: ({ text }) => <RunTextPart text={text} />,
            Reasoning: ({ text }) => <ThinkingBlock text={text} />,
            Empty: ({ status }) =>
              status.type === "running" ? (
                <div className="flex items-center gap-2 text-sm text-muted-foreground">
                  <Loader2 className="size-4 animate-spin" />
                  <span>Thinking...</span>
                </div>
              ) : null,
          }}
        />
      </div>
    </MessagePrimitive.Root>
  );
}

function RunTextPart({ text }: { text: string }): ReactElement {
  return (
    <div className="max-w-none text-sm leading-7 text-foreground" data-testid="run-transcript-text">
      <Markdown>{text}</Markdown>
    </div>
  );
}

function ThinkingBlock({ text }: { text: string }): ReactElement {
  const [open, setOpen] = useState(false);
  return (
    <div className="rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)]">
      <button
        className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left"
        onClick={() => setOpen(value => !value)}
        type="button"
      >
        <span className="flex items-center gap-2 text-xs font-medium text-muted-foreground">
          <Brain className="size-3.5" />
          Thinking
        </span>
        {open ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
      </button>
      {open ? (
        <div className="border-t border-border-subtle px-3 py-2 text-sm leading-6 text-muted-foreground">
          <Markdown>{text}</Markdown>
        </div>
      ) : null}
    </div>
  );
}

function RunToolCard({ part }: { part: ToolCallMessagePartProps }): ReactElement {
  const [open, setOpen] = useState(part.isError || part.status.type === "requires-action");
  const result = toolResultPayload(part.result);
  const Icon = iconForTool(part.toolName);

  return (
    <div
      className={cn(
        "rounded-[var(--radius-md)] border bg-[color:var(--surface-inset)]",
        part.isError ? "border-[color:var(--tone-danger-border)]" : "border-border-subtle"
      )}
      data-testid={`run-transcript-tool-${part.toolCallId}`}
    >
      <button
        className="flex w-full items-center justify-between gap-3 px-3 py-2 text-left"
        onClick={() => setOpen(value => !value)}
        type="button"
      >
        <span className="flex min-w-0 items-center gap-2">
          <Icon className="size-4 shrink-0 text-muted-foreground" />
          <span className="truncate text-sm font-medium text-foreground">{part.toolName}</span>
          <StatusBadge
            tone={part.isError ? "danger" : part.status.type === "running" ? "accent" : "neutral"}
          >
            {toolStatusLabel(part)}
          </StatusBadge>
        </span>
        {open ? <ChevronDown className="size-4" /> : <ChevronRight className="size-4" />}
      </button>
      {open ? (
        <div className="space-y-3 border-t border-border-subtle px-3 py-3">
          <StructuredValue label="Input" value={part.args} />
          {part.isError ? (
            <Alert variant="error">{result.summary || "Tool call failed"}</Alert>
          ) : null}
          {result.blocks.length > 0 ? (
            <div className="space-y-2">
              {result.blocks.map((block, index) => (
                <ContentBlockView block={block} key={index} />
              ))}
            </div>
          ) : result.summary ? (
            <p className="text-sm text-muted-foreground">{result.summary}</p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function RunEventPart({ data }: { data: CompozyEventData }): ReactElement {
  return (
    <div className="rounded-[var(--radius-md)] border border-border-subtle bg-muted px-3 py-2 text-sm">
      <div className="flex items-center gap-2 text-muted-foreground">
        <AlertCircle className="size-4" />
        <span className="font-medium text-foreground">{data.title || data.type || "Runtime"}</span>
        {data.tool_call_state ? (
          <StatusBadge tone="neutral">{data.tool_call_state}</StatusBadge>
        ) : null}
      </div>
      {data.text ? <p className="mt-1 text-muted-foreground">{data.text}</p> : null}
      {Array.isArray(data.blocks) && data.blocks.length > 0 ? (
        <div className="mt-2 space-y-2">
          {data.blocks.map((block, index) => (
            <ContentBlockView block={block} key={index} />
          ))}
        </div>
      ) : null}
    </div>
  );
}

function RunBlockPart({ data }: { data: CompozyBlockData }): ReactElement {
  return <ContentBlockView block={data.block} />;
}

function ContentBlockView({ block }: { block: unknown }): ReactElement {
  if (!isRecord(block)) {
    return <StructuredValue label="Block" value={block} />;
  }
  const type = stringValue(block.type);
  switch (type) {
    case "text":
      return <RunTextPart text={stringValue(block.text)} />;
    case "terminal_output":
      return <TerminalBlock block={block} />;
    case "diff":
      return <DiffBlock block={block} />;
    case "tool_result":
      return <ToolResultBlock block={block} />;
    case "image":
      return <ImageBlock block={block} />;
    default:
      return <StructuredValue label={type || "Block"} value={block} />;
  }
}

function TerminalBlock({ block }: { block: Record<string, unknown> }): ReactElement {
  const command = stringValue(block.command);
  const output = stringValue(block.output);
  const exitCode = numberValue(block.exitCode);
  return (
    <div className="overflow-hidden rounded-[var(--radius-md)] border border-border-subtle bg-[color:var(--surface-inset)]">
      <div className="flex items-center justify-between gap-3 border-b border-border-subtle px-3 py-2 text-xs text-muted-foreground">
        <span className="flex min-w-0 items-center gap-2">
          <Terminal className="size-3.5" />
          <span className="truncate">{command || "terminal output"}</span>
        </span>
        {exitCode !== null ? <span className="font-mono">exit {exitCode}</span> : null}
      </div>
      {output ? (
        <pre className="max-h-72 overflow-auto px-3 py-2 text-xs leading-5 text-foreground whitespace-pre-wrap">
          {output}
        </pre>
      ) : null}
    </div>
  );
}

function DiffBlock({ block }: { block: Record<string, unknown> }): ReactElement {
  const filePath = stringValue(block.filePath);
  const diff = stringValue(block.diff);
  return (
    <div className="overflow-hidden rounded-[var(--radius-md)] border border-border-subtle">
      <div className="flex items-center gap-2 border-b border-border-subtle bg-muted px-3 py-2 text-xs text-muted-foreground">
        <Code2 className="size-3.5" />
        <span className="truncate">{filePath || "diff"}</span>
      </div>
      <pre className="max-h-72 overflow-auto bg-[color:var(--surface-inset)] px-3 py-2 text-xs leading-5 text-foreground whitespace-pre-wrap">
        {diff || "No diff content"}
      </pre>
    </div>
  );
}

function ToolResultBlock({ block }: { block: Record<string, unknown> }): ReactElement {
  const content = stringValue(block.content);
  const isError = Boolean(block.isError);
  return (
    <div
      className={cn(
        "rounded-[var(--radius-md)] border px-3 py-2 text-sm",
        isError
          ? "border-[color:var(--tone-danger-border)] text-[color:var(--tone-danger-text)]"
          : "border-border-subtle text-muted-foreground"
      )}
    >
      {content || "Tool result"}
    </div>
  );
}

function ImageBlock({ block }: { block: Record<string, unknown> }): ReactElement {
  const uri = stringValue(block.uri);
  const data = stringValue(block.data);
  const mimeType = stringValue(block.mimeType);
  const src = uri || (data && mimeType ? `data:${mimeType};base64,${data}` : "");
  if (!src) {
    return <StructuredValue label="Image" value={block} />;
  }
  return (
    <img
      alt="Run transcript image output"
      className="max-h-80 rounded-[var(--radius-md)] border border-border-subtle object-contain"
      src={src}
    />
  );
}

function StructuredValue({ label, value }: { label: string; value: unknown }): ReactElement {
  return (
    <details className="rounded-[var(--radius-md)] border border-border-subtle bg-muted px-3 py-2">
      <summary className="cursor-pointer text-xs font-medium text-muted-foreground">
        {label}
      </summary>
      <pre className="mt-2 max-h-72 overflow-auto text-xs leading-5 text-foreground whitespace-pre-wrap">
        {JSON.stringify(value, null, 2)}
      </pre>
    </details>
  );
}

function convertRunMessagePart(part: RunUIMessagePart): ThreadPart[] {
  switch (part.type) {
    case "text":
      return part.text ? [{ type: "text", text: part.text }] : [];
    case "reasoning":
      return part.text ? [{ type: "reasoning", text: part.text }] : [];
    case "dynamic-tool":
      return [convertToolPart(part)];
    case "data-compozy-event":
    case "data-compozy-block":
      return [{ type: part.type, data: part.data ?? {} }];
    default:
      return [{ type: "data-compozy-block", data: { part } }];
  }
}

function convertToolPart(part: RunUIMessagePart): ThreadPart {
  const input = toJSONObject(part.input);
  const output = part.output ?? null;
  return {
    type: "tool-call",
    toolCallId: part.toolCallId || part.id || `${part.toolName || "tool"}-call`,
    toolName: part.toolName || "tool",
    args: input,
    argsText: JSON.stringify(input),
    result: part.state === "output-available" || part.state === "output-error" ? output : undefined,
    isError: part.state === "output-error" || Boolean(part.errorText),
  };
}

function mergeTranscriptMessages(
  baseMessages: readonly RunUIMessage[],
  liveEvents: readonly RunFeedEvent[]
): RunUIMessage[] {
  const messages = [...baseMessages];
  const seen = new Set(messages.map(message => message.id));
  for (const event of liveEvents) {
    const message = runUIMessageFromLiveEvent(event);
    if (!message || seen.has(message.id)) {
      continue;
    }
    messages.push(message);
    seen.add(message.id);
  }
  return messages;
}

function runUIMessageFromLiveEvent(event: RunFeedEvent): RunUIMessage | null {
  if (event.kind !== "session.update") {
    return null;
  }
  const payload = isRecord(event.payload) ? event.payload : null;
  const update = payload && isRecord(payload.update) ? payload.update : null;
  if (!update) {
    return null;
  }
  const kind = stringValue(update.kind);
  const id = `live-${event.seq ?? event.id}`;
  if (kind === "agent_message_chunk") {
    const text = textFromBlocks(update.blocks);
    return text ? runLiveMessage(id, [{ type: "text", text, state: "done" }]) : null;
  }
  if (kind === "agent_thought_chunk") {
    const text = textFromBlocks(update.thought_blocks);
    return text ? runLiveMessage(id, [{ type: "reasoning", text, state: "done" }]) : null;
  }
  if (kind === "tool_call_started" || kind === "tool_call_updated") {
    const toolCallId = stringValue(update.tool_call_id) || id;
    const toolPart: RunUIMessagePart = {
      type: "dynamic-tool",
      toolName: toolNameFromBlocks(update.blocks),
      toolCallId,
      title: toolNameFromBlocks(update.blocks),
      input: inputFromBlocks(update.blocks),
      output: outputFromBlocks(update.blocks),
      state: toolStateFromLiveUpdate(stringValue(update.tool_call_state)),
    };
    return runLiveMessage(`live-tool-${toolCallId}`, [toolPart]);
  }
  return runLiveMessage(id, [
    {
      type: "data-compozy-event",
      data: {
        type: kind || event.kind,
        text: summarizeLivePayload(event.kind, event.payload),
        blocks: Array.isArray(update.blocks) ? update.blocks : undefined,
      },
    },
  ]);
}

function summarizeLivePayload(kind: string, payload: unknown): string {
  if (!isRecord(payload)) {
    return kind;
  }
  const summary = stringValue(payload.summary) || stringValue(payload.message);
  if (summary) {
    return summary;
  }
  const error = stringValue(payload.error);
  if (error) {
    return error;
  }
  return kind;
}

function runLiveMessage(id: string, parts: RunUIMessagePart[]): RunUIMessage {
  return {
    id,
    role: "assistant",
    parts,
  };
}

function toolStateFromLiveUpdate(state: string): string {
  switch (state) {
    case "pending":
    case "in_progress":
      return "input-streaming";
    case "failed":
      return "output-error";
    case "completed":
      return "output-available";
    case "waiting_for_confirmation":
      return "approval-requested";
    default:
      return "input-available";
  }
}

function textFromBlocks(blocks: unknown): string {
  if (!Array.isArray(blocks)) {
    return "";
  }
  return blocks
    .map(block =>
      isRecord(block) && stringValue(block.type) === "text" ? stringValue(block.text) : ""
    )
    .filter(Boolean)
    .join("");
}

function toolNameFromBlocks(blocks: unknown): string {
  if (!Array.isArray(blocks)) {
    return "tool";
  }
  for (const block of blocks) {
    if (!isRecord(block) || stringValue(block.type) !== "tool_use") {
      continue;
    }
    return stringValue(block.toolName) || stringValue(block.name) || "tool";
  }
  return "tool";
}

function inputFromBlocks(blocks: unknown): Record<string, unknown> | undefined {
  if (!Array.isArray(blocks)) {
    return undefined;
  }
  for (const block of blocks) {
    if (!isRecord(block) || stringValue(block.type) !== "tool_use") {
      continue;
    }
    return isRecord(block.input) ? block.input : undefined;
  }
  return undefined;
}

function outputFromBlocks(blocks: unknown): Record<string, unknown> | undefined {
  if (!Array.isArray(blocks) || blocks.length === 0) {
    return undefined;
  }
  return { blocks };
}

function toolResultPayload(value: unknown): { blocks: unknown[]; summary: string } {
  if (!isRecord(value)) {
    return { blocks: [], summary: typeof value === "string" ? value : "" };
  }
  const blocks = Array.isArray(value.blocks) ? value.blocks : [];
  return { blocks, summary: stringValue(value.summary) };
}

function iconForTool(toolName: string): typeof Wrench {
  const normalized = toolName.toLowerCase();
  if (normalized.includes("bash") || normalized.includes("terminal")) {
    return Terminal;
  }
  if (normalized.includes("read") || normalized.includes("write") || normalized.includes("edit")) {
    return FileText;
  }
  return Wrench;
}

function toolStatusLabel(part: ToolCallMessagePartProps): string {
  if (part.status.type === "running") {
    return "running";
  }
  if (part.status.type === "requires-action") {
    return "waiting";
  }
  return part.isError ? "failed" : "done";
}

function toJSONObject(value: unknown): JSONObject {
  const json = toJSONValue(value);
  return isJSONObject(json) ? json : {};
}

function toJSONValue(value: unknown): JSONValue {
  if (
    value === null ||
    typeof value === "string" ||
    typeof value === "number" ||
    typeof value === "boolean"
  ) {
    return value;
  }
  if (Array.isArray(value)) {
    return value.map(toJSONValue);
  }
  if (isRecord(value)) {
    const result: JSONObject = {};
    for (const [key, item] of Object.entries(value)) {
      result[key] = toJSONValue(item);
    }
    return result;
  }
  return String(value);
}

function isJSONObject(value: JSONValue): value is JSONObject {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === "object" && !Array.isArray(value);
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value : "";
}

function numberValue(value: unknown): number | null {
  return typeof value === "number" ? value : null;
}
