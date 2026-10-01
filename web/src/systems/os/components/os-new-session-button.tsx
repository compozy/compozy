import { Plus } from "lucide-react";

import { Button, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { GLOBAL_SCOPE_COPY } from "@/systems/workspace";

export interface OsNewSessionButtonProps {
  /** A project is active; Global scope has none to start a session in. */
  hasProject: boolean;
  /** Creation is already in flight. */
  busy?: boolean;
  onNewSession: () => void;
  /** Test id on the focusable wrapper of the disabled state. */
  disabledTestId?: string;
}

/**
 * The window-head New session action (secondary, never the inverted primary:
 * heads carry no page CTA). Without a project it stays visible but
 * disabled and says why: a disabled button cannot take focus, so a focusable
 * wrapper carries the tooltip and the accessible name.
 */
export function OsNewSessionButton({
  hasProject,
  busy = false,
  onNewSession,
  disabledTestId,
}: OsNewSessionButtonProps) {
  const button = (
    <Button disabled={!hasProject || busy} onClick={onNewSession} variant="secondary" size="sm">
      <Plus aria-hidden="true" data-icon="inline-start" />
      New session
    </Button>
  );
  if (hasProject) return button;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            aria-label={GLOBAL_SCOPE_COPY.newSessionNeedsProjectName}
            className="inline-flex rounded-pill outline-none focus-visible:shadow-focus-ring"
            data-testid={disabledTestId}
            tabIndex={0}
          />
        }
      >
        {button}
      </TooltipTrigger>
      <TooltipContent>{GLOBAL_SCOPE_COPY.newSessionNeedsProject}</TooltipContent>
    </Tooltip>
  );
}
