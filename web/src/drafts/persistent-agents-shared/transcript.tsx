/** Stand-ins for the session transcript and composer, which need a live session runtime. */
import { ArrowUp, Paperclip } from "lucide-react";
import type { ReactNode } from "react";

import { Button, ChatMessageBubble, cn } from "@compozy/ui";

export function TranscriptColumn({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className="flex min-h-0 flex-1 flex-col overflow-y-auto">
      <div className={cn("mx-auto flex w-full max-w-3xl flex-col gap-4 px-6 py-6", className)}>
        {children}
      </div>
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

export function AgentTurn({ children }: { children: ReactNode }) {
  return <p className="text-transcript-message text-fg">{children}</p>;
}

export function DraftComposer({
  placeholder,
  children,
  disabled = false,
  above,
}: {
  placeholder: string;
  children?: ReactNode;
  disabled?: boolean;
  above?: ReactNode;
}) {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-2 px-6 pt-2 pb-5">
      {above}
      <div className="relative flex flex-col gap-2 rounded-lg bg-surface-2 p-3 shadow-hairline-inset">
        <p className={cn("min-h-6 text-transcript-message", children ? "text-fg" : "text-subtle")}>
          {children ?? placeholder}
        </p>
        <div className="flex items-center justify-between">
          <Button
            aria-label="Attach"
            disabled={disabled}
            size="icon-sm"
            type="button"
            variant="quiet"
          >
            <Paperclip aria-hidden="true" />
          </Button>
          <Button aria-label="Send" disabled={disabled} size="icon-sm" type="button">
            <ArrowUp aria-hidden="true" />
          </Button>
        </div>
      </div>
    </div>
  );
}
