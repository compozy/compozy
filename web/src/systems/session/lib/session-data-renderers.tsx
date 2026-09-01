import type { DataMessagePartProps } from "@assistant-ui/react";

import { ClarificationDataPart } from "../components/clarification-data-part";
import { PermissionDataPart } from "../components/permission-data-part";
import { RuntimeActivityNotice } from "../components/runtime-activity-notice";
import { isClarifyEventData } from "./clarify-event";
import type { AgentEventPayload, CompozyPermissionData } from "../types";

export function CompozyPermissionDataRenderer({
  data,
}: DataMessagePartProps<CompozyPermissionData>) {
  return <PermissionDataPart data={data} />;
}

export function CompozyEventDataRenderer({ data }: DataMessagePartProps<AgentEventPayload>) {
  // A clarify event renders its transcript receipt; everything else keeps the
  // runtime marker renderer. Pending decisions live on the composer dock.
  if (isClarifyEventData(data)) {
    return <ClarificationDataPart data={data} />;
  }
  return <RuntimeActivityNotice event={data} />;
}
