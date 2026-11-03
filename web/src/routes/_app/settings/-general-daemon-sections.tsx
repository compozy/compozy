import type { Dispatch, SetStateAction } from "react";

import { SettingRow, SettingsGroup, type SettingsGeneralSection } from "@/systems/settings";
import { Input, Switch } from "@compozy/ui";

type GeneralConfig = SettingsGeneralSection["config"];

interface DraftSectionProps {
  draft: GeneralConfig;
  setDraft: Dispatch<SetStateAction<GeneralConfig | null>>;
}

/** Memory-usage logging cadence — an operator knob, rendered as a row inside General's Advanced fold. */
export function MemoryReportRow({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingRow
      data-testid="settings-page-general-memory-report-interval"
      help="Applies after CompozyOS restarts."
      description="How often CompozyOS writes its memory use to the logs. 0 turns it off."
      label="Memory usage logging"
      control={
        <Input
          className="w-32 font-mono"
          data-testid="settings-page-general-memory-report-interval-input"
          value={draft.daemon.memory_report_interval}
          placeholder="5m"
          onChange={event =>
            setDraft(prev => {
              const current = prev ?? draft;
              return {
                ...current,
                daemon: { ...current.daemon, memory_report_interval: event.target.value },
              };
            })
          }
        />
      }
    />
  );
}

export function RedactionSection({ draft, setDraft }: DraftSectionProps) {
  return (
    <SettingsGroup data-testid="settings-page-general-redact" title="Privacy">
      <SettingRow
        data-testid="settings-page-general-redact-enabled"
        help="Applies after CompozyOS restarts."
        description="Masks text that looks like a password or API key. Known secrets are always hidden."
        label="Hide passwords and keys in logs"
        control={
          <Switch
            data-testid="settings-page-general-redact-enabled-switch"
            checked={draft.redact.enabled}
            onCheckedChange={checked =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, redact: { ...current.redact, enabled: checked } };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}

/** The PATCH validation message, shown on the row when it names the editor command. */
function editorCommandError(saveError: string | null): string | undefined {
  return saveError?.includes("desktop.editor_command") ? saveError : undefined;
}

/** `desktop.editor_command`: the argv template the desktop app opens transcript files with. */
export function EditorCommandSection({
  draft,
  setDraft,
  saveError,
}: DraftSectionProps & { saveError: string | null }) {
  return (
    <SettingsGroup data-testid="settings-page-general-editor" title="Editor">
      <SettingRow
        data-testid="settings-page-general-editor-command"
        description="Used by the desktop app to open files from the transcript. Use {path}, {line} and {col}. Leave empty to use the system text editor."
        error={editorCommandError(saveError)}
        label="Editor command"
        control={
          <Input
            aria-label="Editor command"
            className="font-mono"
            data-testid="settings-page-general-editor-command-input"
            placeholder="code -g {path}:{line}:{col}"
            value={draft.desktop?.editor_command ?? ""}
            onChange={event =>
              setDraft(prev => {
                const current = prev ?? draft;
                return { ...current, desktop: { editor_command: event.target.value } };
              })
            }
          />
        }
      />
    </SettingsGroup>
  );
}
