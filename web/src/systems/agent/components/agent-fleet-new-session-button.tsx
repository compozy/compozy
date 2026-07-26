import { MessageSquare } from "lucide-react";

import { Button } from "@agh/ui";

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
      title="New session"
      type="button"
      variant="ghost"
    >
      <MessageSquare aria-hidden="true" className="size-3.5" />
    </Button>
  );
}

export { AgentFleetNewSessionButton };
