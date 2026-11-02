/** The head every editor shares: the agent, large and in its current pose, and what this page is. */
import type { AgentAvatarState } from "@compozy/ui";
import type { ReactNode } from "react";

import { agentEditorMark } from "../../lib/agent-editor-mark";
import type { AgentPayload } from "../../types";
import { AgentMark } from "../agent-mark";

export function EditorHead({
  agent,
  title,
  subtitle,
  state = "idle",
  children,
}: {
  agent: AgentPayload;
  title: string;
  subtitle: ReactNode;
  state?: AgentAvatarState;
  children?: ReactNode;
}) {
  return (
    <header className="flex items-center gap-4">
      <AgentMark
        {...agentEditorMark(agent)}
        provider={agent.provider}
        quietIdle
        size={56}
        state={state}
      />
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <h1 className="text-ui-title-lg font-medium text-fg">{title}</h1>
        <p className="truncate text-body text-muted-foreground">{subtitle}</p>
      </div>
      {children}
    </header>
  );
}
