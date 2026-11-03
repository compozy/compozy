import type { AgentAvatarAppearance } from "@compozy/ui";
import { Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import { AgentMark } from "@/systems/agent";

import { SESSION_ROW_MARK_TOOLTIP_DELAY } from "./session-list-row-frame";

export interface SessionListAgentMarkProps {
  sessionId: string;
  label: string;
  appearance: AgentAvatarAppearance;
  /** The mark sits above the row's stretched target; a click activates the row as the target would. */
  onClick?: (event: React.MouseEvent<HTMLElement>) => void;
}

/**
 * The agent's face on a row whose line no longer spells the agent's name: the name rides on a
 * hover tooltip after a short rest. The mark stays out of the a11y tree because the row's
 * accessible description already names the agent.
 */
export function SessionListAgentMark({
  sessionId,
  label,
  appearance,
  onClick,
}: SessionListAgentMarkProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        delay={SESSION_ROW_MARK_TOOLTIP_DELAY}
        render={
          <AgentMark
            aria-hidden="true"
            appearance={appearance}
            className="relative z-10"
            data-testid={`session-agent-mark-${sessionId}`}
            label={label}
            onClick={onClick}
            quietIdle
            size={16}
            state="unknown"
          />
        }
      />
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
