/** The head every editor shares: the agent, large and in its current pose, and what this page is. */
import type { ReactNode } from "react";

import { AgentAvatar } from "../stays-on-agents-shared/agent-avatar";
import type { AgentState } from "../stays-on-agents-shared/agent-state";
import type { DraftAgent } from "../stays-on-agents-shared/fixtures";

export function EditorHead({
  agent,
  title,
  subtitle,
  state = "idle",
  children,
}: {
  agent: DraftAgent;
  title: string;
  subtitle: ReactNode;
  state?: AgentState;
  children?: ReactNode;
}) {
  return (
    <header className="flex items-center gap-4">
      <AgentAvatar
        appearance={agent.appearance}
        label={agent.label}
        provider={agent.provider}
        quietIdle
        size={56}
        state={state}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <h1 className="text-ui-title-lg font-medium text-fg">{title}</h1>
        <p className="truncate text-body text-muted">{subtitle}</p>
      </div>
      {children}
    </header>
  );
}
