import { ArrowRightLeft, GitFork } from "lucide-react";

import { Button, Empty, Separator } from "@compozy/ui";

import type { SessionOriginView } from "../lib/session-origin";

export interface SessionContinueDividerProps {
  origin: SessionOriginView;
  onOpenSource?: (sessionId: string) => void;
}

/**
 * The boundary before a derived child's own first message. The carried context
 * travels inside the first prompt and is never drawn as messages, so one
 * hairline with the pill's words marks where this session starts.
 */
export function SessionContinueDivider({ origin, onOpenSource }: SessionContinueDividerProps) {
  const Glyph = origin.kind === "fork" ? GitFork : ArrowRightLeft;
  const label = `${origin.verb} ${origin.dividerSubject}`;
  const linked = origin.linkable && onOpenSource !== undefined;
  const words = (
    <>
      <Glyph aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      <span className="max-w-[36ch] truncate">{label}</span>
    </>
  );
  return (
    <Separator
      aria-label={label}
      className="pt-2 pb-3.5"
      data-kind={origin.kind}
      data-link={linked ? "true" : "false"}
      data-testid="session-origin-divider"
      label={
        linked ? (
          <Button
            className="h-auto max-w-full gap-1.5 px-0 text-fg-2"
            data-testid="session-origin-divider-link"
            onClick={() => onOpenSource(origin.parentSessionId)}
            size="xs"
            type="button"
            variant="link"
          >
            {words}
          </Button>
        ) : (
          <span className="inline-flex min-w-0 items-center gap-1.5 text-muted">{words}</span>
        )
      }
      labelClassName="flex min-w-0 normal-case tracking-normal"
      lineClassName="bg-line-soft"
    />
  );
}

const EMPTY_CHILD_DESCRIPTION = {
  continue: "The conversation carried over is sent with your first message.",
  fork: "The conversation up to the fork point is carried into your first message.",
} as const;

/** A derived child before its first message: the divider alone, then a compact empty state. */
export function SessionContinueEmptyChild({ origin, onOpenSource }: SessionContinueDividerProps) {
  return (
    <div className="flex flex-col" data-testid="session-thread-empty">
      <SessionContinueDivider onOpenSource={onOpenSource} origin={origin} />
      <Empty
        description={EMPTY_CHILD_DESCRIPTION[origin.kind]}
        fill={false}
        icon={origin.kind === "fork" ? GitFork : ArrowRightLeft}
        size="compact"
        title="Nothing said here yet"
      />
    </div>
  );
}
