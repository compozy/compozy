import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import { SessionListProviderMark } from "../session-list/session-list-provider-mark";
import { subagentModelLabel, subagentNativeKind } from "./subagent-format";
import type { SubagentView } from "./types";

/** Recorded runtime only: absent metadata must not inherit the parent runtime. */
export function SubagentRuntimeMeta({
  subagent,
  providerMark = true,
}: {
  subagent: SubagentView;
  providerMark?: boolean;
}) {
  const { agent, provider, model, reasoning_effort: effort } = subagent.runtime;
  const identity = useAgentIdentityView(agent ?? "");
  const native = subagent.origin === "provider_native";
  const kind = native ? subagentNativeKind(subagent) : agent ? identity.label : null;
  const modelLabel = [model ? subagentModelLabel(model) : null, effort].filter(Boolean).join(" ");
  const label = [provider, modelLabel, kind].filter(Boolean).join(" · ");
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
