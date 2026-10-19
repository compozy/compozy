import { useSessionRuntimeRenderContext } from "@/systems/session/hooks/use-session-runtime-render-context";

/**
 * The workspace whose files and sessions `@` can mention; empty for hosts
 * without a session runtime (the empty-desktop composer), which get no picker.
 */
export function useMentionWorkspaceId(): string {
  return useSessionRuntimeRenderContext()?.workspaceId ?? "";
}
