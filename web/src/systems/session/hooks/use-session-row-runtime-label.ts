import { useQuery } from "@tanstack/react-query";

import { agentsListOptions } from "@/systems/agent";

import { sessionRuntimeProjection } from "../lib/session-runtime-projection";
import type { SessionPayload } from "../types";
import { useSessionRuntimeLabel, type SessionRuntimeLabel } from "./use-session-runtime-label";

/**
 * The row's runtime: what the session runs now (or last ran), falling back to the
 * agent's defaults before any selection, plus the runtime the next message switches
 * to while a saved selection has not reached it yet, so the row and the composer
 * agree on what runs next.
 */
export function useSessionRowRuntimeLabel(session: SessionPayload): SessionRuntimeLabel {
  const { current, next } = sessionRuntimeProjection(session.runtime);
  const defaults = useQuery({
    ...agentsListOptions(session.workspace_id, session.profile_name),
    enabled: !current && !!session.profile_name,
    select: agents => agents.find(agent => agent.name === session.agent_name)?.effective_runtime,
  });
  const runtime = useSessionRuntimeLabel(current ?? defaults.data);
  const pending = useSessionRuntimeLabel(next);
  if (!runtime) return { provider: "unknown", label: "Runtime not selected" };
  if (!pending || !current || !next) return runtime;
  // Labels leave speed out; a pending speed change names it on both sides so they differ.
  if ((current.speed ?? "normal") === (next.speed ?? "normal"))
    return { ...runtime, next: pending };
  return { ...withSpeed(runtime, current.speed), next: withSpeed(pending, next.speed) };
}

/** Appends `Fast` to a label's names; a provider-only label keeps an empty model so the row shows its provider. */
function withSpeed<T extends Omit<SessionRuntimeLabel, "next">>(
  label: T,
  speed: string | undefined
): T {
  if (speed !== "fast") return label;
  return {
    ...label,
    label: `${label.label} Fast`,
    model: label.model ? `${label.model} Fast` : "",
  };
}
