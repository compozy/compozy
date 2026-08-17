import { ExternalLink } from "lucide-react";

import { Button, Pill, Spinner } from "@compozy/ui";

import { settingsUpdateVersionTransition } from "../lib/update-presentation";
import type { SettingsUpdateTrackView } from "../lib/update-presentation";
import type { SettingsUpdateTarget } from "../types";
import { SettingRow, SettingValue } from "./setting-row";

export interface SettingsUpdateTrackRowProps {
  track: SettingsUpdateTrackView;
  onApply: (target: SettingsUpdateTarget) => void;
  onCancel: () => void;
  /** The track an apply request is currently in flight for, if any. */
  pendingTarget: SettingsUpdateTarget | null;
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
 * consequence sentence where it adds truth, the version lane, and the controls
 * the projected track truth allows. Status, action, cancel, and release notes may
 * coexist when each communicates a distinct fact.
 *
 * The apply affordance is absent — never disabled — whenever the daemon says this
 * track is not ours to mutate (SD-007): a managed install, a track with nothing
 * to apply, or a home whose update channel is already held.
 */
export function SettingsUpdateTrackRow({
  track,
  onApply,
  onCancel,
  pendingTarget,
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
          {track.canApply ? (
            <Button
              data-testid={`settings-page-general-update-apply-${track.id}`}
              disabled={pendingTarget !== null}
              onClick={() => onApply(track.id)}
              size="sm"
              type="button"
              variant="neutral"
            >
              {pendingTarget === track.id ? <Spinner className="size-3" /> : null}
              Update {track.id}
            </Button>
          ) : null}
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
