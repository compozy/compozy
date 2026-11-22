import { definePrototype, type PrototypeProps } from "@compozy/proto";

import {
  COMPUTERS_DOWNGRADED,
  COMPUTERS_PRO,
  LOCAL_COMPUTER,
} from "@/systems/computers/mocks/remote-computers-fixtures";

import { RunOnAutomationEditorScene } from "./run-on-automation-editor";
import { RunOnKanbanNewScene } from "./run-on-kanban-new";
import { RunOnSessionCreateScene } from "./run-on-session-create";

export const meta = definePrototype({
  title: "Run on",
  spec: "remote-computers",
  surface: "U4",
  route: "/sessions",
  states: [
    { id: "session-create-connected", label: "Session create · Picker closed on devbox" },
    { id: "picker-open", label: "Picker open · This computer, Auto & fleet load" },
    { id: "offline-draft", label: "Offline target · Draft kept, Start disabled" },
    { id: "reconnect-inline", label: "Reconnect inline · Redial offline host in menu" },
    { id: "needs-pro-row", label: "Downgraded · Needs Pro rows with inline Upgrade" },
    { id: "auto-resolved", label: "Auto resolved · Picks studio with capacity context" },
    { id: "auto-no-eligible", label: "Auto blocked · No computer can run this project" },
    { id: "local-execution-off", label: "Local agents off · This computer omitted" },
    { id: "kanban-new-session", label: "Kanban · New session with compact Run on" },
    { id: "automation-job-editor", label: "Automation editor · No Auto, ownership hint" },
  ],
});

export default function RemoteComputersRunOnDraft({ state }: PrototypeProps) {
  switch (state) {
    case "picker-open":
      return (
        <RunOnSessionCreateScene
          initialProject="api"
          initialTarget="cmp_7f3k2q9x"
          key={state}
          loadBalancingEnabled
          pickerDefaultOpen
        />
      );
    case "offline-draft":
      return (
        <RunOnSessionCreateScene
          initialProject="trainer"
          initialPrompt="Run the 8-GPU batch packing benchmark and compare tokens/sec against main."
          initialTarget="cmp_4b2m9x1p"
          key={state}
          loadBalancingEnabled
        />
      );
    case "reconnect-inline":
      return (
        <RunOnSessionCreateScene
          highlightedReconnectId="cmp_4b2m9x1p"
          initialProject="api"
          initialTarget="cmp_7f3k2q9x"
          key={state}
          loadBalancingEnabled
          pickerDefaultOpen
        />
      );
    case "needs-pro-row":
      return (
        <RunOnSessionCreateScene
          computers={COMPUTERS_DOWNGRADED}
          initialProject="api"
          initialTarget="cmp_1c9d0a2b"
          key={state}
          loadBalancingEnabled={false}
          pickerDefaultOpen
        />
      );
    case "auto-resolved":
      return (
        <RunOnSessionCreateScene
          initialProject="api"
          initialTarget="auto"
          key={state}
          loadBalancingEnabled
        />
      );
    case "auto-no-eligible":
      return (
        <RunOnSessionCreateScene
          initialProject="trainer"
          initialTarget="auto"
          key={state}
          loadBalancingEnabled
        />
      );
    case "local-execution-off":
      return (
        <RunOnSessionCreateScene
          computers={COMPUTERS_PRO}
          initialProject="api"
          initialTarget="cmp_7f3k2q9x"
          key={state}
          loadBalancingEnabled
          localComputer={{ ...LOCAL_COMPUTER, localRunAgents: false }}
          pickerDefaultOpen
        />
      );
    case "kanban-new-session":
      return <RunOnKanbanNewScene initialTarget="cmp_1c9d0a2b" key={state} />;
    case "automation-job-editor":
      return <RunOnAutomationEditorScene initialTarget="cmp_7f3k2q9x" key={state} />;
    default:
      return (
        <RunOnSessionCreateScene
          initialProject="api"
          initialTarget="cmp_7f3k2q9x"
          key={state}
          loadBalancingEnabled
        />
      );
  }
}
