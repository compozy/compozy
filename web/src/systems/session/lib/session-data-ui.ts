import { makeAssistantDataUI } from "@assistant-ui/react";

import type { AgentEventPayload, AghPermissionData } from "../types";
import { AghEventDataRenderer, AghPermissionDataRenderer } from "./session-data-renderers";

export const AghPermissionDataUI = makeAssistantDataUI<AghPermissionData>({
  name: "agh-permission",
  render: AghPermissionDataRenderer,
});

export const AghEventDataUI = makeAssistantDataUI<AgentEventPayload>({
  name: "agh-event",
  render: AghEventDataRenderer,
});
