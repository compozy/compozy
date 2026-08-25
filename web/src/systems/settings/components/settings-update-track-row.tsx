import { ExternalLink } from "lucide-react";

import { Button, Pill, Spinner } from "@compozy/ui";

import { settingsUpdateVersionTransition } from "../lib/update-presentation";
import type { SettingsUpdateTrackView } from "../lib/update-presentation";
import { SettingRow, SettingValue } from "./setting-row";

export interface SettingsUpdateTrackRowProps {
  track: SettingsUpdateTrackView;
  onCancel: () => void;
  isCanceling: boolean;
}

/** `current → latest` while a different build is pending, else the bare version. */
function TrackVersion({ track }: { track: SettingsUpdateTrackView }) {
  const target = settingsUpdateVersionTransition(track);
  return (
    <SettingValue mono>
      {track.currentVersion}
      {target ? (
        <>
          <span aria-hidden="true" className="text-faint">
            {" → "}
          </span>
          <span className="sr-only"> to </span>
          <span className="text-fg">{target}</span>
        </>
      ) : null}
    </SettingValue>
  );
}

/**
 * One update track (runtime or app) as a settings row: label, the daemon's
 * consequence sentence where it adds truth, the version lane, cancel, and release
 * notes. The shared apply action lives in the section header so one click can
 * update every eligible track.
 *
 * `canApply` remains in the view model as the daemon-derived input for that
 * shared action; this row never invents a per-track mutation affordance (SD-007).
 */
export function SettingsUpdateTrackRow({
  track,
  onCancel,
  isCanceling,
}: SettingsUpdateTrackRowProps) {
  // A track with no live progress (settled, staged, refused) keeps its status
  // pill rather than a spinner that would mean nothing.
  const progress = track.progress;

  return (
    <SettingRow
      data-testid={`settings-page-general-update-track-${track.id}`}
      description={track.description ?? undefined}
      label={track.label}
      control={
        <>
          <TrackVersion track={track} />
          {progress ? (
            <span
              aria-atomic="true"
              aria-live="polite"
              className="flex items-center gap-1.5"
              data-testid={`settings-page-general-update-progress-${track.id}`}
              role="status"
            >
              <Spinner aria-hidden="true" className="size-3.5 text-info" role="presentation" />
              <SettingValue mono>
                {/* The phase word is the fact; percent is a qualifier, so it sits
                    one step quieter and disappears when nothing measures it. */}
                <span className="text-fg tabular-nums">{progress.phase}</span>
                {progress.percent === null ? null : (
                  <>
                    <span aria-hidden="true" className="text-faint">
                      {" · "}
                    </span>
                    <span className="tabular-nums">{progress.percent}%</span>
                  </>
                )}
              </SettingValue>
            </span>
          ) : (
            <Pill tone={track.tone}>
              {track.tone === "success" ? <Pill.Dot tone="success" /> : null}
              {track.statusLabel}
            </Pill>
          )}
          {track.canCancel ? (
            <Button
              data-testid="settings-page-general-update-cancel"
              disabled={isCanceling}
              onClick={onCancel}
              size="sm"
              type="button"
              variant="ghost"
            >
              {isCanceling ? <Spinner className="size-3" /> : null}
              Cancel staged update
            </Button>
          ) : null}
          {track.releaseUrl ? (
            <Button
              nativeButton={false}
              render={
                <a
                  aria-label={`Open ${track.id} release notes`}
                  data-testid={`settings-page-general-update-release-${track.id}`}
                  href={track.releaseUrl}
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
  );
}
