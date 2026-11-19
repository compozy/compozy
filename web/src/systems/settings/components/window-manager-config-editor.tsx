import { HelpTip, Slider } from "@compozy/ui";

import type { WindowManagerConfigEditorModel } from "../hooks/use-window-manager-config-editor";
import { WINDOW_MANAGER_RANGES } from "../lib/window-manager-config-ranges";
import { SettingsAdvancedFold, SettingsProvChip } from "./settings-advanced-fold";
import { SettingsGroup } from "./settings-group";
import { SettingsTile, SettingsTiles } from "./settings-tiles";
import { SettingRow } from "./setting-row";
import { WindowManagerBehaviorPicks } from "./layouts/window-manager-behavior-picks";
import { WindowManagerGapEditor } from "./layouts/window-manager-gap-editor";

/**
 * Global window-manager defaults. One save bar for the whole page lives on the
 * route; this composes the sections and nothing else. Shortcuts live on their
 * own page because they apply live instead of joining this draft.
 */
export function WindowManagerConfigEditor({ editor }: { editor: WindowManagerConfigEditorModel }) {
  return (
    <>
      <SettingsGroup description="Takes effect as soon as you save." title="Window behavior">
        <WindowManagerBehaviorPicks draft={editor.draft} setDraft={editor.setDraft} />
      </SettingsGroup>

      <SettingsAdvancedFold data-testid="settings-page-layouts-advanced" padded>
        <SettingsGroup
          bare
          help="Drag the guides. Every value is measured in screen pixels."
          title="Spacing"
        >
          <WindowManagerConfigCard
            help="Drag the four outer guides for screen insets, and the cross in the middle for the space between windows in a split."
            problem={editor.problems.find(problem => problem.field === "gaps")?.message}
            title="Gaps"
          >
            <WindowManagerGapEditor draft={editor.draft} setDraft={editor.setDraft} />
          </WindowManagerConfigCard>
        </SettingsGroup>
        <SettingsGroup title="History">
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
            <SettingsTile label="Scope" value="Everywhere" />
            <SettingsTile detail="No restart needed" label="Lifecycle" value="Live" />
          </SettingsTiles>
        </SettingsGroup>
      </SettingsAdvancedFold>
    </>
  );
}

/** Group a behavior control with its explanation and validation error. */
function WindowManagerConfigCard({
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
      className={`flex flex-col overflow-hidden rounded-lg bg-card shadow-card ${className ?? ""}`}
    >
      <header className="border-b border-line-soft px-3.5 py-3">
        <div className="flex items-center gap-1.5">
          <h3 className="text-body font-medium text-fg">{title}</h3>
          <HelpTip label={`About ${title.toLowerCase()}`}>{help}</HelpTip>
        </div>
      </header>
      <div className="flex flex-1 flex-col gap-3 p-3.5">
        {children}
        {problem ? (
          <p className="text-form-hint text-destructive" role="alert">
            {problem}
          </p>
        ) : null}
      </div>
    </section>
  );
}
