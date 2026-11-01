import { useNavigate } from "@tanstack/react-router";
import { Archive, Bot, HeartPulse, Pause, Play } from "@compozy/ui/icons";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  TopbarOverflowIcon,
} from "@compozy/ui";

import {
  agentInstanceResumeBlocked,
  useAgentInstanceActions,
  type AgentInstancePayload,
} from "@/systems/agent";

/** What you can do to the agent itself, from its session. */
export function StaysOnAgentMenu({ instance }: { instance: AgentInstancePayload }) {
  const navigate = useNavigate();
  const { archive, pause, resume } = useAgentInstanceActions();
  const paused = instance.state === "paused";
  const openAgent = () =>
    void navigate({ to: "/agents/$name", params: { name: instance.agent_name } });
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`${instance.label} actions`}
        render={<Button size="icon-sm" type="button" variant="quiet" />}
      >
        <TopbarOverflowIcon aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-48">
        {paused ? (
          <DropdownMenuItem
            disabled={agentInstanceResumeBlocked(instance)}
            onClick={() => resume(instance.id)}
          >
            <Play aria-hidden="true" />
            Resume
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onClick={() => pause(instance.id)}>
            <Pause aria-hidden="true" />
            Pause
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={openAgent}>
          <Bot aria-hidden="true" />
          Open agent
        </DropdownMenuItem>
        <DropdownMenuItem onClick={openAgent}>
          <HeartPulse aria-hidden="true" />
          Check-ins
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => archive(instance.id)}>
          <Archive aria-hidden="true" />
          Archive
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
