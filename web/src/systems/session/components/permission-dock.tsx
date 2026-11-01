import {
  terminalBlockedRememberedDecisions,
  terminalIdFromDetail,
  terminalPermissionDetail,
  useKnownTerminalTitle,
} from "@/systems/terminal/parts";

import { usePermissionDock } from "../hooks/use-permission-dock";
import { useSession } from "../hooks/use-sessions";
import type { PermissionRequest } from "../types";
import { PermissionDockView } from "./permission-dock-view";

export interface PermissionDockProps {
  enabled?: boolean;
  permission: PermissionRequest;
  sessionId: string;
  workspaceId: string;
  /** "1/N" when more decisions wait behind this one. */
  countLabel: string | null;
  onResolved: () => void;
}

/**
 * The pending permission as a composer-docked approval card: the tool's
 * identity well + the ask, the subject on the sunken inset, then the pill
 * actions with the inverted "Allow once" last. Buttons render only for the
 * decisions the runtime offers; reject-always lives behind the reject split
 * menu on ordinary asks; digit keys 1–4 decide directly (ignoring focused
 * inputs) and each button shows its digit. Exec that always asks withholds
 * both remembered polarities.
 *
 * Host chrome stays Dock — the live decision surface. Terminal facts flavor
 * the body as an attention-row read inside that host.
 */
export function PermissionDock({
  enabled = true,
  permission,
  sessionId,
  workspaceId,
  countLabel,
  onResolved,
}: PermissionDockProps) {
  // The runtime classification governs both the visible actions and keyboard
  // shortcuts. A hidden remembered decision must not remain reachable by key.
  const terminalDetail = terminalPermissionDetail(
    permission.toolId ?? permission.toolName,
    permission.toolInput
  );
  const blockedDecisions = terminalBlockedRememberedDecisions(terminalDetail);
  const { decide, decisionOptions, isResolved, isSubmitting, subject } = usePermissionDock({
    enabled,
    permission,
    sessionId,
    workspaceId,
    onResolved,
    blockedDecisions,
  });
  const session = useSession(sessionId);
  const agentName = session.data?.agent_name?.trim() || "The agent";
  const catalogTitle = useKnownTerminalTitle(
    workspaceId,
    session.data?.profile_name,
    terminalDetail ? terminalIdFromDetail(terminalDetail) : null
  );

  if (isResolved) {
    return null;
  }

  return (
    <PermissionDockView
      agentName={agentName}
      catalogTitle={catalogTitle}
      countLabel={countLabel}
      decide={decide}
      decisionOptions={decisionOptions}
      isSubmitting={isSubmitting}
      permission={permission}
      subject={subject}
      terminalDetail={terminalDetail}
    />
  );
}
