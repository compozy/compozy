import { Button, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { FolderOpen, SquareTerminal } from "@compozy/ui/icons";

import type { ComputerPayload } from "@/systems/computers";

export interface RemoteSessionHeaderControls {
  computer: ComputerPayload;
  disabled: boolean;
  onOpenTerminal: () => void;
  onOpenFiles: () => void;
}

/**
 * Remote-session Terminal and Files header buttons (`_uiux.md` U6, US-011/US-012).
 *
 * Truthfully omitted when this device lacks `terminal:operate` (US-011.EC-1) or
 * `files:read`, and disabled while the remote computer is offline or paused.
 */
export function SessionRemoteTopbarActions({
  remote,
  hasFileRoot,
}: {
  remote: RemoteSessionHeaderControls | null;
  hasFileRoot: boolean;
}) {
  if (!remote) return null;
  const hasTerminalScope = remote.computer.scopes.includes("terminal:operate");
  const showRemoteFiles = remote.computer.scopes.includes("files:read") && hasFileRoot;
  return (
    <>
      {hasTerminalScope ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label={`Terminal on ${remote.computer.name}`}
                data-testid="session-remote-terminal-action"
                disabled={remote.disabled}
                onClick={remote.onOpenTerminal}
                size="icon-sm"
                type="button"
                variant="quiet"
              />
            }
          >
            <SquareTerminal aria-hidden="true" className="size-4" />
          </TooltipTrigger>
          <TooltipContent>Terminal on {remote.computer.name}</TooltipContent>
        </Tooltip>
      ) : null}
      {showRemoteFiles ? (
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                aria-label={`Files on ${remote.computer.name}`}
                data-testid="session-remote-files-action"
                disabled={remote.disabled}
                onClick={remote.onOpenFiles}
                size="icon-sm"
                type="button"
                variant="quiet"
              />
            }
          >
            <FolderOpen aria-hidden="true" className="size-4" />
          </TooltipTrigger>
          <TooltipContent>Files on {remote.computer.name}</TooltipContent>
        </Tooltip>
      ) : null}
    </>
  );
}
