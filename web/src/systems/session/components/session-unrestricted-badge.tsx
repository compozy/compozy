import { Pill, Tooltip, TooltipContent, TooltipTrigger, cn } from "@compozy/ui";

import { SESSION_UNRESTRICTED_COPY } from "../lib/session-unrestricted";

/**
 * Compact warning-tone mark for a session whose agent reads and acts across
 * every project and profile. Session rows show it on line 2 after the agent;
 * the session window shows it in its title trail.
 */
export function SessionUnrestrictedBadge({
  className,
  ...props
}: Omit<React.ComponentProps<"span">, "children">) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <span
            aria-label={`${SESSION_UNRESTRICTED_COPY.label}: ${SESSION_UNRESTRICTED_COPY.tooltip}`}
            className={cn("inline-flex shrink-0", className)}
            data-slot="session-unrestricted-badge"
            {...props}
          >
            <Pill size="xs" tone="warning">
              {SESSION_UNRESTRICTED_COPY.label}
            </Pill>
          </span>
        }
      />
      <TooltipContent>{SESSION_UNRESTRICTED_COPY.tooltip}</TooltipContent>
    </Tooltip>
  );
}
