import type { DataMessagePartProps } from "@assistant-ui/react";

import { ClarificationDataPart } from "../components/clarification-data-part";
import { PermissionDataPart } from "../components/permission-prompt";
import { RuntimeActivityNotice } from "../components/runtime-activity-notice";
import { useSessionRuntimeRenderContext } from "../hooks/use-session-runtime-render-context";
import { isClarifyEventData } from "./clarify-event";
import type { AgentEventPayload, CompozyPermissionData } from "../types";

export function CompozyPermissionDataRenderer({
  data,
}: DataMessagePartProps<CompozyPermissionData>) {
  const renderContext = useSessionRuntimeRenderContext();
  if (!renderContext) {
    throw new Error("CompozyPermissionDataUI requires SessionRuntimeRenderProvider");
  }
  return (
    <PermissionDataPart
      data={data}
      sessionId={renderContext.sessionId}
      workspaceId={renderContext.workspaceId}
    />
  );
}

export function CompozyEventDataRenderer({ data }: DataMessagePartProps<AgentEventPayload>) {
  const renderContext = useSessionRuntimeRenderContext();
  // Ordinary events keep their renderer and never require the provider; only a clarify event routes
  // to the clarification UI, and only when the render context is present.
  if (renderContext && isClarifyEventData(data)) {
    return (
      <ClarificationDataPart
        data={data}
        sessionId={renderContext.sessionId}
        workspaceId={renderContext.workspaceId}
      />
    );
  }
  return <RuntimeActivityNotice event={data} />;
}
