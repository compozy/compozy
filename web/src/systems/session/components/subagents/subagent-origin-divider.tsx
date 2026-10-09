import { Bot } from "lucide-react";

import { Button, Separator } from "@compozy/ui";

import { SUBAGENT_OF_DELETED_SESSION } from "./subagent-format";

export interface SubagentOriginParent {
  id: string;
  title: string;
}

export interface SubagentOriginDividerProps {
  /** The parent as this client reads it; `null` once it was deleted. */
  parent: SubagentOriginParent | null;
  /** Switches this window to the parent. */
  onOpenParent?: (parentSessionId: string) => void;
}

/**
 * The hairline before a subagent's first message (transcript VC-07): the same
 * grammar as the continue/fork divider, reading `Subagent of <parent>` with an
 * `Open parent` action, or `Subagent of a deleted session` with none.
 */
export function SubagentOriginDivider({ parent, onOpenParent }: SubagentOriginDividerProps) {
  const title = parent?.title.trim() || null;
  const label = parent && title ? `Subagent of ${title}` : SUBAGENT_OF_DELETED_SESSION;
  return (
    <Separator
      aria-label={label}
      className="pt-2 pb-3.5"
      data-kind="subagent"
      data-link={parent && onOpenParent ? "true" : "false"}
      data-testid="session-origin-divider"
      label={
        <span className="inline-flex min-w-0 items-center gap-1.5 text-muted">
          <Bot aria-hidden="true" className="size-3 shrink-0 text-subtle" />
          {parent && title ? (
            <>
              <span className="shrink-0">Subagent of</span>
              <span className="max-w-[36ch] truncate text-fg-2">{title}</span>
              {onOpenParent ? (
                <>
                  <span aria-hidden="true" className="text-faint">
                    ·
                  </span>
                  <Button
                    className="h-auto shrink-0 px-0 text-fg-2"
                    data-testid="session-origin-divider-link"
                    onClick={() => onOpenParent(parent.id)}
                    size="xs"
                    type="button"
                    variant="link"
                  >
                    Open parent
                  </Button>
                </>
              ) : null}
            </>
          ) : (
            <span className="truncate">{label}</span>
          )}
        </span>
      }
      labelClassName="flex min-w-0 normal-case tracking-normal"
      lineClassName="bg-line-soft"
    />
  );
}
