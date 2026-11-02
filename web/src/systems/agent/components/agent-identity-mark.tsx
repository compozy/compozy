import type { AgentAvatarProps } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { useAgentIdentityView } from "../hooks/use-agent-identity-view";
import type { AgentIdentityScope } from "../lib/agent-identity-context";
import { AgentMark } from "./agent-mark";

type AgentIdentityMarkProps = Omit<AgentAvatarProps, "appearance" | "label" | "spriteUrl"> & {
  /** The slug. */
  name: string;
  agentScope?: AgentIdentityScope;
};

/** The agent's face from its slug. A state is passed only where the surface knows one. */
export function AgentIdentityMark({ name, agentScope, ...props }: AgentIdentityMarkProps) {
  const identity = useAgentIdentityView(name, agentScope);
  return (
    <AgentMark appearance={identity.appearance} label={identity.label} state="unknown" {...props} />
  );
}

/** The agent's label as text, for lines that already carry a mark or have no room for one. */
export function AgentIdentityLabel({
  name,
  agentScope,
}: {
  name: string;
  agentScope?: AgentIdentityScope;
}) {
  const { label } = useAgentIdentityView(name, agentScope);
  return <>{label}</>;
}

/** Face and label together; the avatar's name repeats the text, so only the text is announced. */
export function AgentIdentityChip({
  name,
  agentScope,
  size = 16,
  className,
  labelClassName,
}: {
  name: string;
  agentScope?: AgentIdentityScope;
  size?: number;
  className?: string;
  labelClassName?: string;
}) {
  const identity = useAgentIdentityView(name, agentScope);
  return (
    <span className={cn("inline-flex min-w-0 items-center gap-1.5", className)}>
      <AgentMark
        aria-hidden="true"
        appearance={identity.appearance}
        label={identity.label}
        quietIdle
        size={size}
        state="unknown"
      />
      <span className={cn("min-w-0 truncate", labelClassName)}>{identity.label}</span>
    </span>
  );
}
