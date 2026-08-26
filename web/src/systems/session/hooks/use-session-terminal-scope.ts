import { useQuery } from "@tanstack/react-query";

import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";
import { sessionDetailOptions } from "../lib/query-options";

/** Resolves the profile-owned terminal scope for the transcript being rendered. */
export function useSessionTerminalScope(): { workspaceId: string; profile: string } | null {
  const runtime = useSessionRuntimeRenderContext();
  const { data: session } = useQuery({
    ...sessionDetailOptions(runtime?.workspaceId ?? "", runtime?.sessionId ?? ""),
    enabled: Boolean(runtime?.workspaceId && runtime?.sessionId),
  });
  return runtime && session?.profile_name
    ? { workspaceId: runtime.workspaceId, profile: session.profile_name }
    : null;
}
