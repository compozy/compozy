import { Button, HelpTip, Pill, Slider } from "@compozy/ui";

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
import { ShortcutPresetCard } from "./layouts/shortcut-preset-card";
import { WindowManagerSnapMap } from "./layouts/window-manager-snap-map";

interface WindowManagerConfigEditorProps {
  editor: WindowManagerConfigEditorModel;
}

/**
 * Global window-manager defaults. One save bar for the whole page lives on the
 * route; this composes the sections and nothing else.
 */
export function WindowManagerConfigEditor({ editor }: WindowManagerConfigEditorProps) {
  const recorder = useWindowManagerShortcutRecorder(
    editor.draft.shortcuts,
    editor.draft.shortcutDefaults,
    editor.setShortcuts
  );
  const changed = Object.keys(editor.draft.shortcuts).length;

  return (
    <>
      <SettingsGroup
        action={<SettingsLiveChip />}
        description="These take effect immediately."
        title="Window behavior"
      >
        <WindowManagerBehaviorPicks draft={editor.draft} setDraft={editor.setDraft} />
      </SettingsGroup>

      <SettingsGroup
        bare
        help="Drag the guides. Every value is measured in screen pixels."
        title="Spacing and snapping"
      >
        <div className="grid gap-3 min-[900px]:grid-cols-2">
          <Card
            help="Drag the four outer guides for screen insets, and the cross in the middle for the space between tiles."
            problem={editor.problems.find(problem => problem.field === "gaps")?.message}
            title="Gaps"
          >
            <WindowManagerGapEditor draft={editor.draft} setDraft={editor.setDraft} />
          </Card>
          <Card
            help="Where a dragged window is caught. Drag a band to resize it; pick what a centre zone claims."
            problem={editor.problems.find(problem => problem.field === "snap")?.message}
            title="Snap zones"
          >
            <WindowManagerSnapMap draft={editor.draft} setDraft={editor.setDraft} />
          </Card>
        </div>
        <Card
          className="mt-3"
          help="Snapping a window to the same edge again cycles it through these widths. Drag a stop to move it, click the track to add one, or press Backspace on a selected stop to remove it."
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
        help="Click a chord to record a new one. Only the ones you change are stored — everything else follows the shipped default."
        title="Shortcuts"
      >
        <ShortcutPresetCard
          defaults={editor.draft.shortcutDefaults}
          overrides={editor.draft.shortcuts}
          onChange={editor.setShortcuts}
        />
        <WindowManagerShortcutTable
          defaults={editor.draft.shortcutDefaults}
          overrides={editor.draft.shortcuts}
          recorder={recorder}
        />
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
          description="Older steps fall off the end."
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
  help,
  problem,
  className,
  children,
}: {
  title: string;
  help: string;
  problem?: string;
  className?: string;
  children: React.ReactNode;
}) {
  return (
    <section
      className={`flex flex-col overflow-hidden rounded-lg border border-line bg-canvas-soft ${className ?? ""}`}
    >
      <header className="border-b border-line-soft px-3.5 py-3">
        <div className="flex items-center gap-1.5">
          <h3 className="text-small-body font-semibold text-fg-strong">{title}</h3>
          <HelpTip label={`About ${title.toLowerCase()}`}>{help}</HelpTip>
        </div>
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
