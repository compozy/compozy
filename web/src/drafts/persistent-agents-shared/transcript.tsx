/**
 * The session pane's two halves from real parts. The composer is the shipped one: its input,
 * action row, send button and model control under a fixture runtime, without the session
 * controller that binds them to a daemon. Turns use the primitives the transcript renders; the
 * thread itself needs a live session, so turns stay plain until the draft graduates.
 */
import {
  AssistantRuntimeProvider,
  ComposerPrimitive,
  useExternalStoreRuntime,
  type AppendMessage,
  type ThreadMessage,
} from "@assistant-ui/react";
import { LexicalComposerInput } from "@assistant-ui/react-lexical";
import { useState, type ReactNode } from "react";

import { ChatMessageBubble, InputGroup, Markdown, StateGlyph, TypingDots, cn } from "@compozy/ui";

import { SessionComposerActionRow } from "@/components/assistant-ui/session-composer-action-row";
import { SessionComposerDirectiveChip } from "@/components/assistant-ui/session-composer-chip";
import { ThreadContentRail } from "@/components/assistant-ui/session-thread-content-rail";
import { RuntimeSelector } from "@/systems/runtime/components/runtime-selector";
import type {
  RuntimeModelOption,
  RuntimeProviderOption,
  RuntimeSelectorValue,
} from "@/systems/runtime/components/runtime-selector";

import { RUNTIME_MODELS, RUNTIME_PROVIDERS } from "./sessions-fixtures";

const THREAD_WIDTH = "mx-auto w-full max-w-3xl";
const ignore = () => undefined;
const NO_MESSAGES: ThreadMessage[] = [];

export function TranscriptColumn({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className={cn(THREAD_WIDTH, "flex flex-col gap-5 px-6 py-6", className)}>{children}</div>
    </div>
  );
}

export function UserTurn({ children }: { children: ReactNode }) {
  return (
    <ChatMessageBubble align="right" messageRole="user">
      {children}
    </ChatMessageBubble>
  );
}

export function AgentTurn({ children }: { children: string }) {
  return (
    <Markdown className="text-transcript-message text-fg" compact="relaxed">
      {children}
    </Markdown>
  );
}

/** The live status line a running turn shows under the transcript. */
export function WorkingLine({ who }: { who: string }) {
  return (
    <p className="flex items-center gap-2 text-small-body text-muted" role="status">
      <StateGlyph size="sm" state="running" />
      {who} is working
      <TypingDots />
    </p>
  );
}

function ComposerRuntime({
  onSend,
  children,
}: {
  onSend?: (text: string) => void;
  children: ReactNode;
}) {
  const runtime = useExternalStoreRuntime({
    messages: NO_MESSAGES,
    onNew: async (message: AppendMessage) => {
      const text = message.content.flatMap(part => (part.type === "text" ? [part.text] : []));
      onSend?.(text.join("\n"));
    },
  });
  return <AssistantRuntimeProvider runtime={runtime}>{children}</AssistantRuntimeProvider>;
}

function ModelControl({ disabled }: { disabled: boolean }) {
  const [value, setValue] = useState<RuntimeSelectorValue>({
    provider: "claude",
    model: "claude-opus-5-5",
    reasoning_effort: "",
  });
  return (
    <RuntimeSelector
      disabled={disabled}
      models={RUNTIME_MODELS as unknown as RuntimeModelOption[]}
      onChange={setValue}
      providers={RUNTIME_PROVIDERS as RuntimeProviderOption[]}
      value={value}
      variant="composer"
    />
  );
}

export function DraftComposer({
  placeholder,
  children,
  disabled = false,
  above,
  onSend,
}: {
  placeholder: string;
  /** Text already in the composer, for the states that show a message mid-write. */
  children?: ReactNode;
  disabled?: boolean;
  /** Decision docks sit here, as `decisionDock` does in the real composer. */
  above?: ReactNode;
  onSend?: (text: string) => void;
}) {
  return (
    <ComposerRuntime onSend={onSend}>
      <div className="bg-canvas" data-testid="composer-shell">
        <ThreadContentRail className={cn(THREAD_WIDTH, "pt-2 pb-5")} inset="px-4">
          <div className="group/composer relative flex min-w-0 flex-col gap-2 px-2">
            {above}
            <ComposerPrimitive.Unstable_TriggerPopoverRoot>
              <InputGroup variant="composer">
                <ComposerPrimitive.Root
                  className="flex min-w-0 flex-col gap-transcript-inline-gap"
                  data-testid="session-composer-stack"
                >
                  {children ? (
                    <p className="min-h-6 text-body leading-relaxed text-fg">{children}</p>
                  ) : (
                    <LexicalComposerInput
                      aria-label="Session prompt"
                      className={cn(
                        "max-h-72 min-h-6 w-full text-body leading-relaxed text-fg",
                        disabled && "opacity-60"
                      )}
                      data-testid="composer-input"
                      directiveChip={SessionComposerDirectiveChip}
                      inert={disabled}
                      placeholder={placeholder}
                      submitMode="enter"
                    />
                  )}
                  <SessionComposerActionRow
                    actionState={{
                      prompt: disabled ? "disabled" : "enabled",
                      enterHint: { enter: "send", modifier: null },
                      controls: { kind: "send" },
                    }}
                    busyInputSteerDelivery={null}
                    handleInterruptAction={ignore}
                    handleQueueAction={ignore}
                    handleSteerAction={ignore}
                    onCancelPrompt={ignore}
                    promptEmbeddedContextCapability="supported"
                    promptImageCapability="supported"
                    runtimeControl={<ModelControl disabled={disabled} />}
                    sessionId="draft-session"
                  />
                </ComposerPrimitive.Root>
              </InputGroup>
            </ComposerPrimitive.Unstable_TriggerPopoverRoot>
          </div>
        </ThreadContentRail>
      </div>
    </ComposerRuntime>
  );
}
