import { RefreshCw } from "lucide-react";

import { Button, Pill, Spinner } from "@compozy/ui";
import {
  SettingRow,
  SettingValue,
  SettingsGroup,
  settingsUpdateApplicableTargets,
  SettingsUpdateTrackRow,
  settingsUpdateTracks,
  settingsUpdateView,
} from "@/systems/settings";
import type {
  SettingsUpdateApplyResult,
  SettingsUpdateCancelResult,
  SettingsUpdateHolder,
  SettingsUpdateStatus,
  SettingsUpdateTargetSet,
} from "@/systems/settings";

export interface GeneralUpdateActions {
  apply: (targets: SettingsUpdateTargetSet) => void;
  cancel: () => void;
  isApplying: boolean;
  isCanceling: boolean;
  /** The daemon's own answer to the last apply — `accepted` or `blocked`. */
  result: SettingsUpdateApplyResult | null;
  /** The daemon's own answer to the last cancel — `canceled` or declined. */
  cancelResult: SettingsUpdateCancelResult | null;
  error: string | null;
}

interface GeneralUpdateSectionProps {
  data?: SettingsUpdateStatus;
  error: unknown;
  isError: boolean;
  isFetching: boolean;
  isLoading: boolean;
  onRetry: () => void;
  actions: GeneralUpdateActions;
}

function RetryButton({ isFetching, onRetry }: { isFetching: boolean; onRetry: () => void }) {
  return (
    <Button
      data-testid="settings-page-general-update-retry"
      disabled={isFetching}
      onClick={onRetry}
      size="sm"
      type="button"
      variant="ghost"
    >
      {isFetching ? (
        <Spinner aria-hidden="true" className="size-3" />
      ) : (
        <RefreshCw aria-hidden="true" className="size-3" />
      )}
      Retry
    </Button>
  );
}

const HOLDER_SURFACE_LABEL: Record<SettingsUpdateHolder["surface"], string> = {
  cli: "the command line",
  daemon: "CompozyOS",
  web: "another browser window",
  shell: "the desktop app",
};

/** Who holds the update channel, in plain words; the process id stays in the tooltip. */
function UpdateHolderValue({ holder }: { holder: SettingsUpdateHolder }) {
  return (
    <span title={`${holder.surface} · PID ${holder.pid}`}>
      <SettingValue>{`In use by ${HOLDER_SURFACE_LABEL[holder.surface]}`}</SettingValue>
    </span>
  );
}

/**
 * True when the Updates group carries something to act on — a failed check, a
 * failed refresh, or a track that is not settled — so the page can lead with it.
 */
export function generalUpdateNeedsAttention(
  props: Pick<GeneralUpdateSectionProps, "data" | "error" | "isError" | "isLoading">
): boolean {
  const view = settingsUpdateView(props);
  if (view.kind === "error") return true;
  if (view.kind !== "snapshot") return false;
  if (view.refreshError) return true;
  return settingsUpdateTracks(view.snapshot).some(
    track => track.tone !== "success" && track.tone !== "neutral"
  );
}

/**
 * Settings → General → Updates: one durable action applies every eligible track,
 * while each row keeps the track state the daemon reports (ADR-006). The section holds no shell
 * awareness — it reads the daemon like any browser client, so it renders
 * identically in the desktop app and in a plain browser.
 */
export function GeneralUpdateSection(props: GeneralUpdateSectionProps) {
  const view = settingsUpdateView(props);
  if (view.kind === "checking") {
    return (
      <SettingsGroup data-testid="settings-page-general-updates" title="Updates">
        <SettingRow
          data-testid="settings-page-general-update-status"
          description="Checking the install method and latest stable release."
          label="CompozyOS version"
          control={
            <>
              <Spinner className="size-3.5 text-info" />
              <SettingValue>Checking…</SettingValue>
            </>
          }
        />
      </SettingsGroup>
    );
  }

  if (view.kind === "error" || view.kind === "unavailable") {
    return (
      <SettingsGroup data-testid="settings-page-general-updates" title="Updates">
        <SettingRow
          data-testid="settings-page-general-update-status"
          description={view.message}
          label="CompozyOS version"
          control={
            <>
              <Pill tone={view.kind === "error" ? "danger" : "warning"}>
                {view.kind === "error" ? "Check failed" : "Unavailable"}
              </Pill>
              <RetryButton isFetching={props.isFetching} onRetry={props.onRetry} />
            </>
          }
        />
      </SettingsGroup>
    );
  }

  const snapshot = view.snapshot;
  const tracks = settingsUpdateTracks(snapshot);
  const applicableTargets = view.refreshError ? null : settingsUpdateApplicableTargets(tracks);
  // A blocked apply is a 200 whose body refuses; it never reads as success.
  const blocked = props.actions.result?.status === "blocked" ? props.actions.result : null;
  const cancelResult = props.actions.cancelResult;

  return (
    <SettingsGroup
      data-testid="settings-page-general-updates"
      title="Updates"
      // A failed refresh is one event for the whole section, so it lives once in
      // the header while every row keeps its last known truth.
      description={
        view.refreshError
          ? `Showing the last known status. Refresh failed: ${view.refreshError}`
          : undefined
      }
      action={
        view.refreshError || applicableTargets ? (
          <>
            {applicableTargets ? (
              <Button
                data-testid="settings-page-general-update-apply"
                disabled={props.actions.isApplying}
                onClick={() => props.actions.apply(applicableTargets)}
                size="sm"
                type="button"
                variant="neutral"
              >
                {props.actions.isApplying ? <Spinner className="size-3" /> : null}
                Update CompozyOS
              </Button>
            ) : null}
            {view.refreshError ? (
              <>
                <Pill tone="danger">Refresh failed</Pill>
                <RetryButton isFetching={props.isFetching} onRetry={props.onRetry} />
              </>
            ) : null}
          </>
        ) : null
      }
    >
      {tracks.map(track => (
        <SettingsUpdateTrackRow
          key={track.id}
          isCanceling={props.actions.isCanceling}
          onCancel={props.actions.cancel}
          track={track}
        />
      ))}
      {blocked ? (
        <SettingRow
          data-testid="settings-page-general-update-blocked"
          description={blocked.message}
          label="Update channel busy"
          control={blocked.holder ? <UpdateHolderValue holder={blocked.holder} /> : undefined}
        />
      ) : null}
      {cancelResult ? (
        <SettingRow
          data-testid="settings-page-general-update-cancel-result"
          description={cancelResult.message}
          label={cancelResult.status === "canceled" ? "Update canceled" : "Cancel declined"}
          control={
            cancelResult.holder ? (
              <UpdateHolderValue holder={cancelResult.holder} />
            ) : (
              <Pill tone={cancelResult.status === "canceled" ? "neutral" : "warning"}>
                {cancelResult.status === "canceled" ? "Canceled" : "Declined"}
              </Pill>
            )
          }
        />
      ) : null}
      {props.actions.error ? (
        <SettingRow
          data-testid="settings-page-general-update-action-error"
          description={<span className="text-danger">{props.actions.error}</span>}
          label="Update request failed"
        />
      ) : null}
      {tracks.map(track =>
        track.restoredVersion ? (
          <SettingRow
            key={`${track.id}-rollback`}
            data-testid="settings-page-general-update-rollback"
            label={`${track.label} restored version`}
            control={<SettingValue mono>{track.restoredVersion}</SettingValue>}
          />
        ) : null
      )}
      {tracks.map(track =>
        track.lastError ? (
          <SettingRow
            key={`${track.id}-error`}
            data-testid="settings-page-general-update-last-error"
            description={<span className="text-danger">{track.lastError}</span>}
            label={`${track.label} last error`}
          />
        ) : null
      )}
    </SettingsGroup>
  );
}
