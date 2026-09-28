import { MessageSquare } from "lucide-react";

import { Button, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

export interface AgentFleetNewSessionButtonProps {
  agentName: string;
  disabled?: boolean;
  onNewSession: (agentName: string) => void;
}

function AgentFleetNewSessionButton({
  agentName,
  disabled = false,
  onNewSession,
}: AgentFleetNewSessionButtonProps) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            aria-label={`New session with ${agentName}`}
            data-testid={`agent-fleet-new-session-${agentName}`}
            disabled={disabled}
            onClick={event => {
              event.preventDefault();
              event.stopPropagation();
              onNewSession(agentName);
            }}
            size="icon-sm"
            type="button"
            variant="ghost"
          >
            <MessageSquare aria-hidden="true" className="size-3.5" />
          </Button>
        }
      />
      <TooltipContent>Start a session</TooltipContent>
    </Tooltip>
  );
}

export { AgentFleetNewSessionButton };
