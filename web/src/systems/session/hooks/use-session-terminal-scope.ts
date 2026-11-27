import { useQuery } from "@tanstack/react-query";

import { useCurrentComputerId } from "@/hooks/use-current-computer-id";
import type { ComputerId } from "@/lib/computer-scope";

import { useSessionRuntimeRenderContext } from "./use-session-runtime-render-context";
import { sessionDetailOptions } from "../lib/query-options";

/** Resolves the profile-owned terminal scope for the transcript being rendered. */
export function useSessionTerminalScope(): {
  workspaceId: string;
  profile: string;
  computerId: ComputerId;
} | null {
  const runtime = useSessionRuntimeRenderContext();
  const computerId = useCurrentComputerId();
  const { data: session } = useQuery({
    ...sessionDetailOptions(runtime?.workspaceId ?? "", runtime?.sessionId ?? "", computerId),
    enabled: Boolean(runtime?.workspaceId && runtime?.sessionId),
  });
  return runtime && session?.profile_name
    ? { workspaceId: runtime.workspaceId, profile: session.profile_name, computerId }
    : null;
}
