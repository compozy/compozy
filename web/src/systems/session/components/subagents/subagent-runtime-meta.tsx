import { SessionListProviderMark } from "../session-list/session-list-provider-mark";
import { SessionListRowWho } from "../session-list/session-list-row-meta";
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
  const modelLabel = [model, effort].filter(Boolean).join(" · ");
  const label = [provider, agent, modelLabel].filter(Boolean).join(" · ");
  return (
    <span
      className="flex min-w-0 items-center gap-1 text-transcript-caption text-subtle"
      title={label}
    >
      {providerMark && (
        <SessionListProviderMark
          sessionId={subagent.id}
          runtime={provider ? { provider, label } : null}
        />
      )}
      <span className="sr-only">{label}</span>
      <SessionListRowWho
        session={{ agent_name: agent ?? "", badge: "idle" }}
        meta={{ attention: [], notes: modelLabel ? [modelLabel] : [] }}
      />
    </span>
  );
}
