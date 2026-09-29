import { RefreshCw } from "lucide-react";
import type { ReactNode } from "react";

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
  SettingsUpdateTrackView,
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

function UpdatesGroup({
  description,
  action,
  children,
}: {
  description?: ReactNode;
  action?: ReactNode;
  children: ReactNode;
}) {
  return (
    <SettingsGroup
      action={action}
      data-testid="settings-page-general-updates"
      description={description}
      title="Updates"
    >
      {children}
    </SettingsGroup>
  );
}

function UpdateCheckingGroup() {
  return (
    <UpdatesGroup>
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
    </UpdatesGroup>
  );
}

function UpdateFailureGroup({
  failed,
  message,
  isFetching,
  onRetry,
}: {
  failed: boolean;
  message: string;
  isFetching: boolean;
  onRetry: () => void;
}) {
  return (
    <UpdatesGroup>
      <SettingRow
        data-testid="settings-page-general-update-status"
        description={message}
        label="CompozyOS version"
        control={
          <>
            <Pill tone={failed ? "danger" : "warning"}>
              {failed ? "Check failed" : "Unavailable"}
            </Pill>
            <RetryButton isFetching={isFetching} onRetry={onRetry} />
          </>
        }
      />
    </UpdatesGroup>
  );
}

function UpdateGroupAction({
  applicableTargets,
  refreshError,
  actions,
  isFetching,
  onRetry,
}: {
  applicableTargets: SettingsUpdateTargetSet | null;
  refreshError: string | null;
  actions: GeneralUpdateActions;
  isFetching: boolean;
  onRetry: () => void;
}) {
  return (
    <>
      {applicableTargets ? (
        <Button
          data-testid="settings-page-general-update-apply"
          disabled={actions.isApplying}
          onClick={() => actions.apply(applicableTargets)}
          size="sm"
          type="button"
          variant="neutral"
        >
          {actions.isApplying ? <Spinner className="size-3" /> : null}
          Update CompozyOS
        </Button>
      ) : null}
      {refreshError ? (
        <>
          <Pill tone="danger">Refresh failed</Pill>
          <RetryButton isFetching={isFetching} onRetry={onRetry} />
        </>
      ) : null}
    </>
  );
}

function UpdateBlockedRow({ result }: { result: SettingsUpdateApplyResult | null }) {
  // A blocked apply is a 200 whose body refuses; it never reads as success.
  if (result?.status !== "blocked") return null;
  return (
    <SettingRow
      data-testid="settings-page-general-update-blocked"
      description={result.message}
      label="Update channel busy"
      control={result.holder ? <UpdateHolderValue holder={result.holder} /> : undefined}
    />
  );
}

function UpdateCancelResultRow({ result }: { result: SettingsUpdateCancelResult | null }) {
  if (!result) return null;
  const canceled = result.status === "canceled";
  return (
    <SettingRow
      data-testid="settings-page-general-update-cancel-result"
      description={result.message}
      label={canceled ? "Update canceled" : "Cancel declined"}
      control={
        result.holder ? (
          <UpdateHolderValue holder={result.holder} />
        ) : (
          <Pill tone={canceled ? "neutral" : "warning"}>{canceled ? "Canceled" : "Declined"}</Pill>
        )
      }
    />
  );
}

function UpdateTrackFootnotes({ tracks }: { tracks: SettingsUpdateTrackView[] }) {
  return (
    <>
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
    </>
  );
}

/**
 * Settings → General → Updates: one durable action applies every eligible track,
 * while each row keeps the track state the daemon reports (ADR-006). The section holds no shell
 * awareness — it reads the daemon like any browser client, so it renders
 * identically in the desktop app and in a plain browser.
 */
export function GeneralUpdateSection(props: GeneralUpdateSectionProps) {
  const { actions, isFetching, onRetry } = props;
  const view = settingsUpdateView(props);
  if (view.kind === "checking") return <UpdateCheckingGroup />;
  if (view.kind !== "snapshot") {
    return (
      <UpdateFailureGroup
        failed={view.kind === "error"}
        isFetching={isFetching}
        message={view.message}
        onRetry={onRetry}
      />
    );
  }

  const { refreshError } = view;
  const tracks = settingsUpdateTracks(view.snapshot);
  const applicableTargets = refreshError ? null : settingsUpdateApplicableTargets(tracks);

  return (
    <UpdatesGroup
      // A failed refresh is one event for the whole section, so it lives once in
      // the header while every row keeps its last known truth.
      description={
        refreshError ? `Showing the last known status. Refresh failed: ${refreshError}` : undefined
      }
      action={
        refreshError || applicableTargets ? (
          <UpdateGroupAction
            actions={actions}
            applicableTargets={applicableTargets}
            isFetching={isFetching}
            onRetry={onRetry}
            refreshError={refreshError}
          />
        ) : null
      }
    >
      {tracks.map(track => (
        <SettingsUpdateTrackRow
          key={track.id}
          isCanceling={actions.isCanceling}
          onCancel={actions.cancel}
          track={track}
        />
      ))}
      <UpdateBlockedRow result={actions.result} />
      <UpdateCancelResultRow result={actions.cancelResult} />
      {actions.error ? (
        <SettingRow
          data-testid="settings-page-general-update-action-error"
          description={<span className="text-danger">{actions.error}</span>}
          label="Update request failed"
        />
      ) : null}
      <UpdateTrackFootnotes tracks={tracks} />
    </UpdatesGroup>
  );
}
