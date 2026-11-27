import { RefreshCw, Scale, TriangleAlert, WifiOff } from "@compozy/ui/icons";

import { Alert, AlertActions, AlertDescription, Button, cn, Pill, Spinner } from "@compozy/ui";

import type { AutoPlacementCandidateView } from "../lib/placement-view";
import { ComputerLoadMeter } from "./computer-load-meter";

export interface RunOnNoticesProps {
  /** Project name shown inside Auto placement panels and warnings. */
  projectName?: string | null;
  /** Candidates shown when `Run on: Auto` resolved to an eligible computer. */
  autoCandidates?: readonly AutoPlacementCandidateView[];
  /** True when `Run on: Auto` resolved to `409 no_eligible_computer` (UT-106). */
  autoNoEligible?: boolean;
  /** Plain-language explanation paired with `No computer can run this project.`. */
  autoNoEligibleReason?: string | null;
  /** Offline computer selected for drafting (`_uiux.md` U4, UT-083). */
  offlineComputer?: { id: string; name: string } | null;
  isReconnectingOffline?: boolean;
  onReconnectOffline?: (computerId: string) => void;
  /** Shown when local execution is off and no enabled remote computer is selectable. */
  localExecutionBlocked?: boolean;
}

/**
 * Inline status notices for the `Run on` picker inside Session Create and Kanban New session
 * (`_uiux.md` U4, `remote-computers-05-run-on` parity).
 */
export function RunOnNotices({
  projectName,
  autoCandidates,
  autoNoEligible = false,
  autoNoEligibleReason,
  offlineComputer = null,
  isReconnectingOffline = false,
  onReconnectOffline,
  localExecutionBlocked = false,
}: RunOnNoticesProps) {
  if (!autoCandidates?.length && !autoNoEligible && !offlineComputer && !localExecutionBlocked) {
    return null;
  }

  return (
    <div className="flex flex-col gap-3">
      {autoCandidates?.length ? (
        <RunOnCandidates candidates={autoCandidates} projectName={projectName} />
      ) : null}

      {autoNoEligible ? (
        <Alert data-testid="auto-no-eligible-alert" variant="warning">
          <TriangleAlert aria-hidden="true" className="size-4" />
          <AlertDescription>
            <span className="font-medium text-fg">No computer can run this project.</span>{" "}
            {autoNoEligibleReason ||
              "Pick another project or choose a specific computer to start this session."}
          </AlertDescription>
        </Alert>
      ) : null}

      {offlineComputer ? (
        <Alert data-testid="session-create-offline-notice" variant="warning">
          <WifiOff aria-hidden="true" className="size-4" />
          <AlertDescription>
            <span className="font-medium text-fg">{offlineComputer.name} is offline.</span> Your
            draft is kept here; reconnect or pick another computer to start this session.
          </AlertDescription>
          {onReconnectOffline ? (
            <AlertActions>
              <Button
                disabled={isReconnectingOffline}
                onClick={() => onReconnectOffline(offlineComputer.id)}
                size="xs"
                type="button"
                variant="secondary"
              >
                {isReconnectingOffline ? (
                  <Spinner aria-hidden="true" data-icon="inline-start" />
                ) : (
                  <RefreshCw aria-hidden="true" data-icon="inline-start" />
                )}
                {isReconnectingOffline ? "Reconnecting…" : "Reconnect"}
              </Button>
            </AlertActions>
          ) : null}
        </Alert>
      ) : null}

      {localExecutionBlocked ? (
        <Alert data-testid="local-execution-disabled-alert" variant="warning">
          <TriangleAlert aria-hidden="true" className="size-4" />
          <AlertDescription>
            <span className="font-medium text-fg">
              Running agents on this computer is turned off.
            </span>{" "}
            Connect another computer or turn local execution back on in Settings → Computers.
          </AlertDescription>
        </Alert>
      ) : null}
    </div>
  );
}

function RunOnCandidates({
  candidates,
  projectName,
}: {
  candidates: readonly AutoPlacementCandidateView[];
  projectName?: string | null;
}) {
  return (
    <div
      className="flex flex-col gap-2 rounded-md border border-line-soft bg-sunken px-3 py-2.5"
      data-testid="auto-resolution-panel"
    >
      <div className="flex items-center gap-1.5 text-eyebrow font-medium text-fg">
        <Scale aria-hidden="true" className="size-3.5 text-subtle" />
        <span>Eligible computers for {projectName || "this project"}</span>
      </div>
      <div className="flex flex-col gap-1.5 border-t border-line-soft pt-2">
        {candidates.map(candidate => (
          <div
            className={cn(
              "flex items-center justify-between gap-2 text-eyebrow",
              candidate.winner ? "text-fg" : "text-muted-foreground"
            )}
            key={candidate.id}
          >
            <span className="flex min-w-0 items-center gap-1.5 truncate">
              <span className={cn("truncate", candidate.winner && "font-medium")}>
                {candidate.name}
              </span>
              {candidate.winner ? (
                <Pill form="tint" size="sm" tone="success">
                  Chosen
                </Pill>
              ) : null}
              <span className="shrink-0 text-badge text-faint">{candidate.weightLabel}</span>
            </span>
            <ComputerLoadMeter computerName={candidate.name} load={candidate.load} />
          </div>
        ))}
      </div>
    </div>
  );
}
