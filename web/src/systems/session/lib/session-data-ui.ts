import { makeAssistantDataUI } from "@assistant-ui/react";

import type { AgentEventPayload, CompozyPermissionData } from "../types";
import { CompozyEventDataRenderer, CompozyPermissionDataRenderer } from "./session-data-renderers";

export const CompozyPermissionDataUI = makeAssistantDataUI<CompozyPermissionData>({
  name: "compozy-permission",
  render: CompozyPermissionDataRenderer,
});

export const CompozyEventDataUI = makeAssistantDataUI<AgentEventPayload>({
  name: "compozy-event",
  render: CompozyEventDataRenderer,
});
