import { Button, Pill, Slider } from "@agh/ui";

import type { WindowManagerConfigEditorModel } from "../hooks/use-window-manager-config-editor";
import { useWindowManagerShortcutRecorder } from "../hooks/use-window-manager-shortcut-recorder";
import { WINDOW_MANAGER_RANGES } from "../lib/window-manager-snap-geometry";
import { SettingsAdvancedFold, SettingsProvChip } from "./settings-advanced-fold";
import { SettingsGroup } from "./settings-group";
import { SettingsLiveChip } from "./settings-live-chip";
import { SettingsTile, SettingsTiles } from "./settings-tiles";
import { SettingRow } from "./setting-row";
import { WindowManagerBehaviorPicks } from "./layouts/window-manager-behavior-picks";
import { WindowManagerGapEditor } from "./layouts/window-manager-gap-editor";
import { WindowManagerRatioTrack } from "./layouts/window-manager-ratio-track";
import { WindowManagerShortcutTable } from "./layouts/window-manager-shortcut-table";
import { WindowManagerSnapMap } from "./layouts/window-manager-snap-map";

interface WindowManagerConfigEditorProps {
  editor: WindowManagerConfigEditorModel;
}

/**
 * Global window-manager defaults. One save bar for the whole page lives on the
 * route; this composes the sections and nothing else.
 */
export function WindowManagerConfigEditor({ editor }: WindowManagerConfigEditorProps) {
  const recorder = useWindowManagerShortcutRecorder(editor.draft.shortcuts, editor.setShortcuts);
  const changed = Object.keys(editor.draft.shortcuts).length;

  return (
    <>
      <SettingsGroup
        action={<SettingsLiveChip />}
        description="Defaults for every workspace. These take effect immediately."
        title="Window behavior"
      >
        <WindowManagerBehaviorPicks draft={editor.draft} setDraft={editor.setDraft} />
      </SettingsGroup>

      <SettingsGroup
        bare
        description="Drag the guides. Every value is measured in screen pixels."
        title="Spacing and snapping"
      >
        <div className="grid gap-3 min-[900px]:grid-cols-2">
          <Card
            description="Drag the four outer guides for screen insets, and the cross in the middle for the space between tiles."
            problem={editor.problems.find(problem => problem.field === "gaps")?.message}
            title="Gaps"
          >
            <WindowManagerGapEditor draft={editor.draft} setDraft={editor.setDraft} />
          </Card>
          <Card
            description="Where a dragged window is caught. Drag a band to resize it; pick what a centre zone claims."
            problem={editor.problems.find(problem => problem.field === "snap")?.message}
            title="Snap zones"
          >
            <WindowManagerSnapMap draft={editor.draft} setDraft={editor.setDraft} />
          </Card>
        </div>
        <Card
          className="mt-3"
          description="Snapping a window to the same edge again cycles it through these widths. Drag a stop to move it, click the track to add one, or press Backspace on a selected stop to remove it."
          problem={editor.problems.find(problem => problem.field === "repeatRatios")?.message}
          title="Repeat widths"
        >
          <WindowManagerRatioTrack draft={editor.draft} setDraft={editor.setDraft} />
        </Card>
      </SettingsGroup>

      <SettingsGroup
        action={
          <>
            <Pill size="xs" tone={changed === 0 ? "neutral" : "accent"}>
              {changed === 0 ? "All default" : `${changed} changed`}
            </Pill>
            <Button
              disabled={changed === 0}
              size="sm"
              type="button"
              variant="outline"
              onClick={recorder.resetAll}
            >
              Restore defaults
            </Button>
          </>
        }
        description="Click a chord to record a new one. Only the ones you change are stored — everything else follows the shipped default."
        title="Shortcuts"
      >
        <WindowManagerShortcutTable overrides={editor.draft.shortcuts} recorder={recorder} />
      </SettingsGroup>

      <SettingsAdvancedFold data-testid="settings-page-layouts-advanced">
        <SettingRow
          control={
            <div className="flex items-center gap-3">
              <Slider
                aria-label="Layout history steps"
                className="w-44"
                max={WINDOW_MANAGER_RANGES.historyLimit.max}
                min={WINDOW_MANAGER_RANGES.historyLimit.min}
                value={editor.draft.historyLimit}
                onValueChange={historyLimit =>
                  editor.setDraft(current => ({ ...current, historyLimit }))
                }
              />
              <span className="w-16 shrink-0 font-mono text-form-hint tabular-nums text-fg">
                {editor.draft.historyLimit} steps
              </span>
            </div>
          }
          description="How many undo and redo steps each workspace keeps. Older steps fall off the end."
          error={editor.problems.find(problem => problem.field === "historyLimit")?.message}
          label={
            <>
              Layout history <SettingsProvChip>history_limit</SettingsProvChip>
            </>
          }
        />
        <SettingsTiles className="p-4">
          <SettingsTile label="Config section" mono value="[window_manager]" />
          <SettingsTile label="Scope" value="Global only" />
          <SettingsTile detail="Hot-applied, no restart" label="Lifecycle" value="Live" />
        </SettingsTiles>
      </SettingsAdvancedFold>
    </>
  );
}

function Card({
  title,
  description,
  problem,
  className,
  children,
}: {
  title: string;
  description: string;
  problem?: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section
      className={`flex flex-col overflow-hidden rounded-lg border border-line bg-canvas-soft ${className ?? ""}`}
    >
      <header className="border-b border-line-soft px-3.5 py-3">
        <h3 className="text-small-body font-semibold text-fg-strong">{title}</h3>
        <p className="mt-0.5 text-form-label leading-normal text-muted">{description}</p>
      </header>
      <div className="flex flex-1 flex-col gap-3 p-3.5">
        {children}
        {problem ? (
          <p className="text-form-hint text-danger" role="alert">
            {problem}
          </p>
        ) : null}
      </div>
    </section>
  );
}
