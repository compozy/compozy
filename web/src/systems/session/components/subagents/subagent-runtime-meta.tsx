import { SessionListProviderMark } from "../session-list/session-list-provider-mark";
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
      {modelLabel ? (
        <span aria-hidden="true" className="min-w-0 shrink truncate">
          {[model?.replace(/^claude-/, ""), effort].filter(Boolean).join(" · ")}
        </span>
      ) : null}
      {agent ? (
        <span aria-hidden="true" className="min-w-0 shrink-[10000] truncate text-muted-foreground">
          {model || effort ? "· " : ""}
          {agent}
        </span>
      ) : null}
    </span>
  );
}
