import { definePrototype, type PrototypeProps } from "@protohq/cli";

import { COMPUTERS_PRO } from "@/systems/computers/mocks/remote-computers-fixtures";
import {
  DEVBOX_NARROWED_SCOPES,
  DEVBOX_TRANSCRIPT,
  REMOTE_SESSION_ENTRIES,
  VOLUME_SESSION_ENTRIES,
  WORKSTATION_OFFLINE_TRANSCRIPT,
} from "@/systems/computers/mocks/remote-computers-fixtures-sessions";

import { RemoteSessionsWindow } from "../remote-computers-shared-sessions/remote-sessions-window";
import { RemoteKanbanView } from "./remote-kanban-view";
import { RemoteSearchOverlayScene } from "./remote-search-overlay";

export const meta = definePrototype({
  title: "Sessions across computers",
  spec: "remote-computers",
  surface: "U5 U6",
  route: "/sessions",
  states: [
    { id: "unified-remote-session", label: "Sidebar & header · Active session on devbox" },
    { id: "session-header-chip-open", label: "Header chip · Route, load & scopes popover" },
    { id: "remote-offline-session", label: "Offline computer · Dimmed rows & locked composer" },
    { id: "remote-entitlement-paused", label: "Pro ended · Mid-session remote access paused" },
    { id: "per-computer-loading-error", label: "Per-computer · Loading skeleton & inline Retry" },
    { id: "grouped-project-computer", label: "Group by project · Separate project · computer" },
    { id: "kanban-unified", label: "Kanban · Unified board with computer labels" },
    { id: "kanban-grouped-lanes", label: "Kanban · Swimlanes grouped by project · computer" },
    { id: "search-cross-computer", label: "Search · Cross-computer results with provenance" },
    { id: "remote-narrowed-scopes", label: "Narrowed scopes · Terminal omitted on devbox" },
    { id: "volume-scale", label: "Volume · 8 computers, 30+ sessions & paging" },
  ],
});

const DEVBOX_COMPUTER = COMPUTERS_PRO[2]!;
const WORKSTATION_COMPUTER = COMPUTERS_PRO[4]!;
const DEVBOX_SESSION = REMOTE_SESSION_ENTRIES[2]!;
const WORKSTATION_SESSION = REMOTE_SESSION_ENTRIES[6]!;

export default function RemoteComputersSessionsDraft({ state }: PrototypeProps) {
  switch (state) {
    case "session-header-chip-open":
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_COMPUTER}
          activeSession={DEVBOX_SESSION}
          computerChipOpen
          isWorking
          key={state}
          sessions={REMOTE_SESSION_ENTRIES}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
    case "remote-offline-session":
      return (
        <RemoteSessionsWindow
          activeComputer={WORKSTATION_COMPUTER}
          activeSession={WORKSTATION_SESSION}
          isWorking={false}
          key={state}
          sessions={REMOTE_SESSION_ENTRIES}
          transcript={WORKSTATION_OFFLINE_TRANSCRIPT}
        />
      );
    case "remote-entitlement-paused":
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_COMPUTER}
          activeSession={DEVBOX_SESSION}
          entitlementPaused
          isWorking={false}
          key={state}
          sessions={REMOTE_SESSION_ENTRIES}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
    case "per-computer-loading-error":
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_COMPUTER}
          activeSession={DEVBOX_SESSION}
          failedComputers={[
            {
              computerId: "cmp_8e7f6d5c",
              computerName: "gpu-vm",
              message: "Couldn't load sessions from gpu-vm",
            },
          ]}
          isWorking
          key={state}
          loadingComputers={[
            {
              computerId: "cmp_1c9d0a2b",
              computerName: "studio",
              iconKind: "desktop",
              platform: "darwin_arm64",
              rows: 3,
            },
          ]}
          perComputerMore={[
            { computerId: "cmp_7f3k2q9x", computerName: "devbox", remainingCount: 14 },
          ]}
          sessions={REMOTE_SESSION_ENTRIES.filter(entry => entry.computerId !== "cmp_1c9d0a2b")}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
    case "grouped-project-computer":
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_COMPUTER}
          activeSession={DEVBOX_SESSION}
          initialGroupBy="project"
          isWorking
          key={state}
          sessions={REMOTE_SESSION_ENTRIES}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
    case "kanban-unified":
      return <RemoteKanbanView initialGrouped={false} key={state} />;
    case "kanban-grouped-lanes":
      return <RemoteKanbanView initialGrouped key={state} />;
    case "search-cross-computer":
      return <RemoteSearchOverlayScene initialQuery="api" key={state} />;
    case "remote-narrowed-scopes":
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_NARROWED_SCOPES}
          activeSession={DEVBOX_SESSION}
          computerChipOpen
          isWorking
          key={state}
          sessions={REMOTE_SESSION_ENTRIES}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
    case "volume-scale":
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_COMPUTER}
          activeSession={DEVBOX_SESSION}
          isWorking
          key={state}
          perComputerMore={[
            { computerId: "cmp_1c9d0a2b", computerName: "studio", remainingCount: 18 },
            { computerId: "cmp_7f3k2q9x", computerName: "devbox", remainingCount: 24 },
          ]}
          sessions={VOLUME_SESSION_ENTRIES}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
    default:
      return (
        <RemoteSessionsWindow
          activeComputer={DEVBOX_COMPUTER}
          activeSession={DEVBOX_SESSION}
          isWorking
          key={state}
          sessions={REMOTE_SESSION_ENTRIES}
          transcript={DEVBOX_TRANSCRIPT}
        />
      );
  }
}
