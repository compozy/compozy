import {
  KindIcon,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
  providerKindIconRegistry,
} from "@compozy/ui";

import type { SessionPayload } from "../../types";

export interface SessionListProviderMarkProps {
  session: SessionPayload;
  className?: string;
}

/** `provider · model` of the bound runtime; `null` until the session's first bind. */
function sessionRuntimeLabel(session: SessionPayload): { provider: string; label: string } | null {
  const effective = session.runtime?.effective;
  const provider = effective?.provider.trim() ?? "";
  if (provider === "") return null;
  const model = effective?.model?.trim();
  return { provider, label: model ? `${provider} · ${model}` : provider };
}

/**
 * The provider a row runs on, read from its effective runtime so a continue,
 * fork, or process replacement repaints it with the next catalog read. An
 * unbound session has no runtime yet and renders nothing rather than a guess.
 *
 * The mark is neutral ink: rows stay uncolored at rest, so brand inks (Claude's
 * orange is the accent's needs-you hue) drop to gray while keeping their shape.
 */
export function SessionListProviderMark({ session, className }: SessionListProviderMarkProps) {
  const runtime = sessionRuntimeLabel(session);
  if (runtime === null) return null;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <KindIcon
            role="img"
            aria-label={runtime.label}
            data-testid={`session-provider-mark-${session.id}`}
            kind={runtime.provider}
            registry={providerKindIconRegistry}
            size="xs"
            tone="muted"
            className={cn("grayscale", className)}
          />
        }
      />
      <TooltipContent>{runtime.label}</TooltipContent>
    </Tooltip>
  );
}
