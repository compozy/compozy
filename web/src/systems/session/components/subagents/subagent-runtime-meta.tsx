import { useAgentIdentityView } from "@/systems/agent/hooks/use-agent-identity-view";

import { SessionListAgentMark } from "../session-list/session-list-agent-mark";
import { SessionListProviderMark } from "../session-list/session-list-provider-mark";
import { SESSION_ROW_MARK_TOOLTIP_DELAY } from "../session-list/session-list-row-frame";
import { subagentModelLabel, subagentNativeKind } from "./subagent-row-format";
import type { SubagentView } from "./types";

/** Recorded runtime only: absent metadata must not inherit the parent runtime. */
export function SubagentRuntimeMeta({
  subagent,
  providerMark = true,
  quiet = false,
  children,
}: {
  subagent: SubagentView;
  providerMark?: boolean;
  /**
   * A session row's quiet who-line: the agent's face and the provider mark name themselves on a
   * resting hover, and only the model is written.
   */
  quiet?: boolean;
  /** A quiet row's needs-you words, written in the model's place. */
  children?: React.ReactNode;
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
      {quiet && agent ? (
        <SessionListAgentMark
          appearance={identity.appearance}
          label={identity.label}
          sessionId={subagent.id}
        />
      ) : null}
      {providerMark && (
        <SessionListProviderMark
          sessionId={subagent.id}
          runtime={provider ? { provider, label } : null}
          delay={quiet ? SESSION_ROW_MARK_TOOLTIP_DELAY : undefined}
        />
      )}
      <span className="sr-only">{label}</span>
      {children ?? null}
      {modelLabel && !(quiet && children) ? (
        <span aria-hidden="true" className="min-w-0 shrink truncate text-fg-2">
          {modelLabel}
        </span>
      ) : null}
      {modelLabel && kind && !quiet ? (
        <span aria-hidden="true" className="text-subtle">
          ·
        </span>
      ) : null}
      {kind && !quiet ? (
        <span aria-hidden="true" className="min-w-0 shrink-[10000] truncate">
          {kind}
        </span>
      ) : null}
    </span>
  );
}
