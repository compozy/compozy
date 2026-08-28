import { useSessionInspectorState, useSessionRuntimeRenderContext } from "@/systems/session";

export function useOpenSessionCallsPanel(): () => void {
  const context = useSessionRuntimeRenderContext();
  const inspector = useSessionInspectorState(context?.sessionId);
  return () => {
    inspector.openTab("calls");
  };
}
