import { useState } from "react";

import {
  KindIcon,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  providerKindIconRegistry,
} from "@compozy/ui";

import type { SessionRuntimeLabel } from "../../hooks/use-session-runtime-label";

export interface SessionListProviderMarkProps {
  sessionId: string;
  /** The row's resolved current, recorded, selected, or default runtime. */
  runtime: SessionRuntimeLabel | null;
  /**
   * Set while the row holds keyboard focus, fresh for every focus, so the
   * tooltip a keyboard user dismissed with Escape stays closed only for that focus.
   */
  keyboardFocus?: number | null;
  /**
   * The glyph sits above the row's stretched target so mouse hover reaches its
   * tooltip; a click on it activates the row as the target would.
   */
  onClick?: (event: React.MouseEvent<HTMLElement>) => void;
}

/**
 * The row resolves current and recorded identity before next-prompt defaults.
 *
 * The glyph is decorative and never a tab stop: the row's accessible name
 * carries `Provider · Model` as text, and the tooltip opens on the glyph's hover
 * or on the row's keyboard focus. Brand inks stay on, as in the runtime selector,
 * so providers tell apart at a glance; the needs-you state never relies on hue
 * alone (its word and haloed mark carry it).
 */
export function SessionListProviderMark({
  sessionId,
  runtime,
  keyboardFocus = null,
  onClick,
}: SessionListProviderMarkProps) {
  const [hovered, setHovered] = useState(false);
  const [dismissedFocus, setDismissedFocus] = useState<number | null>(null);
  if (runtime === null) return null;
  const open = hovered || (keyboardFocus !== null && keyboardFocus !== dismissedFocus);
  return (
    <Tooltip
      open={open}
      onOpenChange={next => {
        setHovered(next);
        if (!next && keyboardFocus !== null) setDismissedFocus(keyboardFocus);
      }}
    >
      <TooltipTrigger
        render={
          <KindIcon
            aria-hidden="true"
            data-testid={`session-provider-mark-${sessionId}`}
            kind={runtime.provider}
            registry={providerKindIconRegistry}
            size="xs"
            tone="muted"
            className="relative z-10 shrink-0"
            onClick={onClick}
          />
        }
      />
      <TooltipContent>{runtime.label}</TooltipContent>
    </Tooltip>
  );
}
