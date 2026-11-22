import { useState } from "react";
import {
  ArrowLeft,
  Check,
  RefreshCw,
  Send,
  ShieldCheck,
  Terminal,
  WifiOff,
  X,
} from "@compozy/ui/icons";

import {
  Button,
  cn,
  CodeBlock,
  Marker,
  Pill,
  Receipt,
  StateGlyph,
  StatusDot,
  Surface,
  Textarea,
  toast,
  ToolCallRow,
  TypingDots,
} from "@compozy/ui";

import type { UnifiedSessionFixture } from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  PHONE_LIVE_TRANSCRIPT,
  PHONE_PENDING_COMMAND,
  type PhoneTranscriptEntry,
} from "@/systems/computers/mocks/remote-computers-fixtures-phone";
import { WorkspaceBadge } from "@/systems/workspace/components/workspace-badge";

export interface PhoneSessionViewProps {
  session: UnifiedSessionFixture;
  computerName: string;
  offline?: boolean;
  wide?: boolean;
  canOperateTerminal: boolean;
  onBack?: () => void;
  onOpenTerminal?: () => void;
  onRetryConnection?: () => void;
}

/**
 * Session transcript + inline approval + composer on the computer's hosted route.
 * Adapts cleanly between single-pane phone (<1024 px) and two-pane desktop (≥1024 px).
 */
export function PhoneSessionView({
  session,
  computerName,
  offline = false,
  wide = false,
  canOperateTerminal,
  onBack,
  onOpenTerminal,
  onRetryConnection,
}: PhoneSessionViewProps) {
  const [entries, setEntries] = useState<PhoneTranscriptEntry[]>([...PHONE_LIVE_TRANSCRIPT]);
  const [approvalDecision, setApprovalDecision] = useState<"pending" | "allowed" | "denied">(
    session.state === "attention" ? "pending" : "allowed"
  );
  const [draft, setDraft] = useState("");

  const handleSend = () => {
    const trimmed = draft.trim();
    if (!trimmed || offline) return;
    setEntries(current => [
      ...current,
      {
        id: `msg-user-${current.length + 1}`,
        kind: "user",
        text: trimmed,
        timestamp: "now",
      },
    ]);
    setDraft("");
    toast.success(`Sent to ${computerName}`);
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col bg-canvas" data-slot="phone-session-view">
      <header
        className={cn(
          "flex h-14 shrink-0 items-center gap-2 border-b border-line-soft py-2",
          wide ? "px-5" : "px-2.5"
        )}
      >
        {!wide && onBack ? (
          <Button
            aria-label="Back to sessions"
            className="shrink-0"
            onClick={onBack}
            size="icon"
            variant="quiet"
          >
            <ArrowLeft className="size-4" />
          </Button>
        ) : null}

        <div className="flex min-w-0 flex-1 flex-col justify-center gap-0.5">
          <div className="flex items-center gap-1.5">
            <StateGlyph
              size="sm"
              state={offline ? "idle" : approvalDecision === "pending" ? "attention" : "running"}
              still={offline}
            />
            <h1 className="truncate text-body font-medium text-fg-strong">{session.title}</h1>
          </div>
          <div className="flex items-center gap-1.5 text-micro text-subtle">
            <WorkspaceBadge
              identity={{ id: session.project, kind: "workspace", name: session.project }}
            />
            <span aria-hidden="true">·</span>
            <span>{session.model}</span>
            <span aria-hidden="true">·</span>
            <span className="inline-flex items-center gap-1">
              <StatusDot size="sm" tone={offline ? "warning" : "success"} />
              {offline ? `${computerName} · Offline` : computerName}
            </span>
          </div>
        </div>

        {canOperateTerminal && onOpenTerminal ? (
          <Button
            aria-label={`Open terminal on ${computerName}`}
            disabled={offline}
            onClick={onOpenTerminal}
            size={wide ? "sm" : "icon"}
            variant="quiet"
          >
            <Terminal className="size-4" />
            {wide ? <span>Terminal</span> : null}
          </Button>
        ) : null}
      </header>

      {offline ? (
        <div
          className={cn(
            "flex shrink-0 items-center justify-between gap-2 border-b border-line-soft bg-warning-tint py-2",
            wide ? "px-5" : "px-3.5"
          )}
          data-slot="phone-offline-strip"
          role="status"
        >
          <div className="flex min-w-0 items-center gap-2 text-eyebrow text-fg">
            <WifiOff aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
            <span className="truncate">{computerName} is offline. Reconnecting…</span>
          </div>
          <Button className="shrink-0" onClick={onRetryConnection} size="xs" variant="secondary">
            <RefreshCw className="size-3" />
            Retry now
          </Button>
        </div>
      ) : null}

      <div className="min-h-0 flex-1 overflow-y-auto">
        <div
          className={cn(
            "mx-auto flex w-full max-w-3xl flex-col gap-3.5 py-4",
            wide ? "px-6" : "px-4",
            offline && "opacity-75"
          )}
        >
          {entries.map(entry => {
            if (entry.kind === "user") {
              return (
                <div className="flex justify-end" key={entry.id}>
                  <div className="max-w-[86%] rounded-xl bg-chat-fill-user px-3.5 py-2.5 text-small-body leading-relaxed text-fg">
                    <p>{entry.text}</p>
                    <span className="mt-1 block text-right font-mono text-micro text-faint">
                      {entry.timestamp}
                    </span>
                  </div>
                </div>
              );
            }

            if (entry.kind === "tool") {
              return (
                <ToolCallRow
                  defaultExpanded
                  key={entry.id}
                  preview={entry.preview}
                  previewVariant="chip"
                  status={entry.toolStatus ?? "success"}
                  toolName={entry.toolName ?? "Ran"}
                >
                  {entry.toolOutput ? (
                    <ToolCallRow.Output>
                      <CodeBlock
                        code={entry.toolOutput}
                        copyable={false}
                        density="compact"
                        wrapLines
                      />
                    </ToolCallRow.Output>
                  ) : null}
                </ToolCallRow>
              );
            }

            if (entry.kind === "marker") {
              return (
                <Marker
                  icon={<ShieldCheck className="size-3" />}
                  key={entry.id}
                  tone={approvalDecision === "pending" ? "warning" : "neutral"}
                >
                  <span className="flex min-w-0 flex-wrap items-baseline gap-x-1.5">
                    <span className="shrink-0">{entry.text}</span>
                    {entry.command ? (
                      <code
                        className="inline-block max-w-full truncate whitespace-nowrap font-mono text-transcript-body text-muted-foreground"
                        title={entry.command}
                      >
                        {entry.command}
                      </code>
                    ) : null}
                  </span>
                </Marker>
              );
            }

            return (
              <div className="flex flex-col gap-1" key={entry.id}>
                <p className="text-transcript-message leading-relaxed text-fg">{entry.text}</p>
                <span className="font-mono text-micro text-faint">{entry.timestamp}</span>
              </div>
            );
          })}

          {approvalDecision === "allowed" ? (
            <>
              <Receipt icon={<Check className="size-3" />} tone="allowed">
                Allowed once: <code>{PHONE_PENDING_COMMAND.command}</code>
              </Receipt>
              {!offline ? (
                <div className="flex items-center gap-2 px-1 text-eyebrow text-subtle">
                  <TypingDots />
                  <span>Running tests on {computerName}…</span>
                </div>
              ) : null}
            </>
          ) : null}

          {approvalDecision === "denied" ? (
            <Receipt icon={<X className="size-3" />} tone="rejected">
              Denied: <code>{PHONE_PENDING_COMMAND.command}</code>
            </Receipt>
          ) : null}
        </div>
      </div>

      {approvalDecision === "pending" && !offline ? (
        <div className="shrink-0 border-t border-line-soft bg-sunken/70 py-3">
          <div className={cn("mx-auto w-full max-w-3xl", wide ? "px-6" : "px-3")}>
            <Surface className="flex flex-col gap-2.5 p-3.5" variant="card">
              <div className="flex items-center justify-between gap-2">
                <Pill form="plain" tone="attn">
                  <Pill.Dot />
                  Needs approval
                </Pill>
                <span className="truncate font-mono text-micro text-subtle">
                  {PHONE_PENDING_COMMAND.cwd}
                </span>
              </div>
              <CodeBlock
                code={PHONE_PENDING_COMMAND.command}
                copyable={false}
                density="compact"
                language="bash"
                showPrompt
              />
              <p className="text-eyebrow text-muted-foreground">{PHONE_PENDING_COMMAND.reason}</p>
              <div
                className={cn("pt-0.5", wide ? "flex justify-end gap-2" : "grid grid-cols-2 gap-2")}
              >
                <Button
                  className={wide ? undefined : "w-full"}
                  onClick={() => {
                    setApprovalDecision("denied");
                    toast("Command denied");
                  }}
                  size={wide ? "default" : "lg"}
                  variant="outline"
                >
                  Deny
                </Button>
                <Button
                  className={wide ? undefined : "w-full"}
                  onClick={() => {
                    setApprovalDecision("allowed");
                    toast.success(`Approved on ${computerName}`);
                  }}
                  size={wide ? "default" : "lg"}
                >
                  Allow once
                </Button>
              </div>
            </Surface>
          </div>
        </div>
      ) : null}

      <footer className="shrink-0 border-t border-line-soft bg-canvas pt-2.5 pb-3.5">
        <div
          className={cn("mx-auto flex w-full max-w-3xl items-end gap-2", wide ? "px-6" : "px-3")}
        >
          <Textarea
            aria-label={
              offline ? `${computerName} is offline` : `Message session on ${computerName}`
            }
            className="max-h-28 min-h-10 resize-none rounded-lg py-2.5"
            disabled={offline}
            onChange={event => setDraft(event.target.value)}
            onKeyDown={event => {
              if (event.key === "Enter" && !event.shiftKey) {
                event.preventDefault();
                handleSend();
              }
            }}
            placeholder={offline ? `${computerName} is offline` : `Message on ${computerName}…`}
            rows={1}
            value={offline ? "" : draft}
          />
          <Button
            aria-label="Send message"
            className="size-10 shrink-0"
            disabled={offline || draft.trim() === ""}
            onClick={handleSend}
            size="icon-lg"
          >
            <Send className="size-4" />
          </Button>
        </div>
      </footer>
    </div>
  );
}
