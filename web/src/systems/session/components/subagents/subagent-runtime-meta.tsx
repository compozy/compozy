import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import { SessionListProviderMark } from "../session-list/session-list-provider-mark";
import { subagentRuntimeWords } from "./subagent-row-format";
import type { SubagentView } from "./types";

/** Recorded runtime only: absent metadata must not inherit the parent runtime. */
export function SubagentRuntimeMeta({
  subagent,
  providerMark = true,
}: {
  subagent: SubagentView;
  providerMark?: boolean;
}) {
  const { agent, provider } = subagent.runtime;
  const identity = useAgentIdentityView(agent ?? "");
  const { modelLabel, kind, label } = subagentRuntimeWords(subagent, identity.label);
  return (
    <span
      className="flex min-w-0 items-center gap-1 text-transcript-meta text-muted-foreground"
      title={label}
    >
      {providerMark && (
        <SessionListProviderMark
          sessionId={subagent.id}
          runtime={provider ? { provider, label } : null}
        />
      )}
      <span className="sr-only">{label}</span>
      {modelLabel ? (
        <span aria-hidden="true" className="min-w-0 shrink truncate text-fg-2">
          {modelLabel}
        </span>
      ) : null}
      {modelLabel && kind ? (
        <span aria-hidden="true" className="text-subtle">
          ·
        </span>
      ) : null}
      {kind ? (
        <span aria-hidden="true" className="min-w-0 shrink-[10000] truncate">
          {kind}
        </span>
      ) : null}
    </span>
  );
}
