import { useState } from "react";

import {
  KindIcon,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  providerKindIconRegistry,
} from "@compozy/ui";

import { useSessionRuntimeLabel } from "../../hooks/use-session-runtime-label";
import type { SessionPayload } from "../../types";

export interface SessionListProviderMarkProps {
  session: SessionPayload;
  /**
   * Set while the row holds keyboard focus, fresh for every focus, so the
   * tooltip a keyboard user dismissed with Escape stays closed only for that focus.
   */
  keyboardFocus?: number | null;
}

/**
 * The provider a row runs on, read from its effective runtime so a continue,
 * fork, or process replacement repaints it with the next catalog read. An
 * unbound session has no runtime yet and renders nothing rather than a guess.
 *
 * The glyph is decorative and never a tab stop: the row's accessible name
 * carries `Provider · Model` as text, and the tooltip opens on the glyph's hover
 * or on the row's keyboard focus. Neutral ink keeps rows uncolored at rest, so
 * brand inks (Claude's orange is the accent's needs-you hue) drop to gray.
 */
export function SessionListProviderMark({
  session,
  keyboardFocus = null,
}: SessionListProviderMarkProps) {
  const runtime = useSessionRuntimeLabel(session.runtime?.effective);
  const [hovered, setHovered] = useState(false);
  const [dismissedFocus, setDismissedFocus] = useState<number | null>(null);
  if (runtime === null) return null;
  const open = hovered || (keyboardFocus !== null && keyboardFocus !== dismissedFocus);
  return (
    <>
      <span className="sr-only">{runtime.label}</span>
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
              data-testid={`session-provider-mark-${session.id}`}
              kind={runtime.provider}
              registry={providerKindIconRegistry}
              size="xs"
              tone="muted"
              className="grayscale"
            />
          }
        />
        <TooltipContent>{runtime.label}</TooltipContent>
      </Tooltip>
    </>
  );
}
