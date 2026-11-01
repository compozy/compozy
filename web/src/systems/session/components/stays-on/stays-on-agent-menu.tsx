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
  agentInstanceBinding,
  agentInstanceResumeBlocked,
  useAgentInstanceActions,
  type AgentInstancePayload,
} from "@/systems/agent";

/** What you can do to the agent itself, from its session. */
export function StaysOnAgentMenu({ instance }: { instance: AgentInstancePayload }) {
  const navigate = useNavigate();
  const { archive, pause, resume } = useAgentInstanceActions();
  const paused = instance.state === "paused";
  const search = agentInstanceBinding(instance);
  const name = instance.agent_name;
  const openAgent = () => void navigate({ to: "/agents/$name", params: { name }, search });
  const openCheckIns = () =>
    void navigate({ to: "/agents/$name/$page", params: { name, page: "check-ins" }, search });
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
            onClick={() => resume(instance)}
          >
            <Play aria-hidden="true" />
            Resume
          </DropdownMenuItem>
        ) : (
          <DropdownMenuItem onClick={() => pause(instance)}>
            <Pause aria-hidden="true" />
            Pause
          </DropdownMenuItem>
        )}
        <DropdownMenuItem onClick={openAgent}>
          <Bot aria-hidden="true" />
          Open agent
        </DropdownMenuItem>
        <DropdownMenuItem onClick={openCheckIns}>
          <HeartPulse aria-hidden="true" />
          Check-ins
        </DropdownMenuItem>
        <DropdownMenuSeparator />
        <DropdownMenuItem onClick={() => archive(instance)}>
          <Archive aria-hidden="true" />
          Archive
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
