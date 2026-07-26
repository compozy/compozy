import { ExternalLink, RefreshCw } from "lucide-react";

import { Button, Pill, Spinner } from "@agh/ui";
import { SettingRow, SettingValue, SettingsGroup } from "@/systems/settings";
import { settingsUpdateView } from "@/systems/settings/lib/update-presentation";
import type { SettingsUpdateStatus } from "@/systems/settings";

interface GeneralUpdateSectionProps {
  data?: SettingsUpdateStatus;
  error: unknown;
  isError: boolean;
  isFetching: boolean;
  isLoading: boolean;
  onRetry: () => void;
}

function updatePill(snapshot: SettingsUpdateStatus) {
  switch (snapshot.status) {
    case "available":
      return <Pill tone="info">Update available</Pill>;
    case "current":
    case "updated":
      return (
        <Pill tone="success">
          <Pill.Dot tone="success" />
          Up to date
        </Pill>
      );
    case "failed":
      return <Pill tone="danger">Update failed</Pill>;
    case "deferred":
      return <Pill tone="warning">Update deferred</Pill>;
    case "unsupported":
      return <Pill tone="neutral">Unsupported</Pill>;
  }
}

export function GeneralUpdateSection(props: GeneralUpdateSectionProps) {
  const view = settingsUpdateView(props);
  if (view.kind === "checking") {
    return (
      <SettingsGroup title="Updates">
        <SettingRow
          data-testid="settings-page-general-update-status"
          description="Checking the daemon's install method and latest stable release."
          label="AGH version"
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
      <SettingsGroup title="Updates">
        <SettingRow
          data-testid="settings-page-general-update-status"
          description={view.message}
          label="AGH version"
          control={
            <>
              <Pill tone={view.kind === "error" ? "danger" : "warning"}>
                {view.kind === "error" ? "Check failed" : "Unavailable"}
              </Pill>
              <Button
                data-testid="settings-page-general-update-retry"
                disabled={props.isFetching}
                onClick={props.onRetry}
                size="sm"
                type="button"
                variant="ghost"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={props.isFetching ? "size-3 animate-spin" : "size-3"}
                />
                Retry
              </Button>
            </>
          }
        />
        {view.kind === "error" ? (
          <SettingRow
            data-testid="settings-page-general-update-last-error"
            description={<span className="text-danger">{view.message}</span>}
            label="Last error"
          />
        ) : null}
      </SettingsGroup>
    );
  }

  const snapshot = view.snapshot;
  const lastError = snapshot.last_error ?? view.refreshError;
  const installDescription = snapshot.managed
    ? `Installed with ${snapshot.install_method || "a package manager"}. Updates are managed for you.`
    : `Direct-binary install detected. ${snapshot.recommendation ?? "Use the AGH updater for new releases."}`;
  return (
    <SettingsGroup title="Updates">
      <SettingRow
        data-testid="settings-page-general-update-status"
        description={
          view.refreshError
            ? `Showing the last known status. Refresh failed: ${view.refreshError}`
            : installDescription
        }
        label="AGH version"
        control={
          <>
            <SettingValue mono>{snapshot.current_version}</SettingValue>
            {view.refreshError ? <Pill tone="danger">Refresh failed</Pill> : updatePill(snapshot)}
            {view.refreshError ? (
              <Button
                data-testid="settings-page-general-update-retry"
                disabled={props.isFetching}
                onClick={props.onRetry}
                size="sm"
                type="button"
                variant="ghost"
              >
                <RefreshCw
                  aria-hidden="true"
                  className={props.isFetching ? "size-3 animate-spin" : "size-3"}
                />
                Retry
              </Button>
            ) : null}
            {snapshot.release_url ? (
              <Button
                nativeButton={false}
                render={
                  <a
                    aria-label="Open release notes"
                    data-testid="settings-page-general-update-release-link"
                    href={snapshot.release_url}
                    rel="noreferrer"
                    target="_blank"
                  />
                }
                size="sm"
                variant="ghost"
              >
                <ExternalLink aria-hidden="true" className="size-3 text-subtle" />
              </Button>
            ) : null}
          </>
        }
      />
      {snapshot.recommendation ? (
        <SettingRow
          data-testid="settings-page-general-update-recommendation"
          description="Exact command or package-manager path for this install."
          label="Next action"
          control={<SettingValue mono>{snapshot.recommendation}</SettingValue>}
        />
      ) : null}
      {lastError ? (
        <SettingRow
          data-testid="settings-page-general-update-last-error"
          description={<span className="text-danger">{lastError}</span>}
          label="Last error"
        />
      ) : null}
    </SettingsGroup>
  );
}
