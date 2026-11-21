import { ArrowRightLeft, GitFork } from "@compozy/ui/icons";

import { Button, Empty, Separator } from "@compozy/ui";

import type { SessionOriginView } from "../lib/session-origin";

export interface SessionContinueDividerProps {
  origin: SessionOriginView;
  onOpenSource?: (sessionId: string) => void;
}

/**
 * The boundary before a derived child's own first message: one hairline with
 * the pill's words. What the child inherited is drawn above it as a collapsed,
 * read-only block read from the source (#1885); everything below is its own.
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
          <span className="inline-flex min-w-0 items-center gap-1.5 text-muted-foreground">
            {words}
          </span>
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

/** A native clone already holds the source's conversation in the agent's own session. */
const NATIVE_CLONE_DESCRIPTION =
  "The agent already holds the conversation up to the fork point. Your first message continues it.";

export interface SessionContinueEmptyChildProps extends SessionContinueDividerProps {
  /** The child was seeded by a native clone rather than the carried replay. */
  nativeClone?: boolean;
}

/** A derived child before its first message: the divider, then a compact empty state. */
export function SessionContinueEmptyChild({
  origin,
  onOpenSource,
  nativeClone = false,
}: SessionContinueEmptyChildProps) {
  return (
    <div className="flex flex-col" data-testid="session-thread-empty">
      <SessionContinueDivider onOpenSource={onOpenSource} origin={origin} />
      <Empty
        description={
          nativeClone && origin.kind === "fork"
            ? NATIVE_CLONE_DESCRIPTION
            : EMPTY_CHILD_DESCRIPTION[origin.kind]
        }
        fill={false}
        icon={origin.kind === "fork" ? GitFork : ArrowRightLeft}
        size="compact"
        title="Nothing said here yet"
      />
    </div>
  );
}
