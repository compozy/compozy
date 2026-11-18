import type { ReactNode } from "react";

import type { WindowManagerSettingsSection } from "@/systems/os";
import {
  SettingsGroup,
  SettingsLiveChip,
  SettingsPageFrame,
  SettingsPageState,
  useSettingsTopbar,
  useShortcutsSettingsData,
  useWindowManagerKeyboardEditors,
  WindowManagerGlobalHotkeys,
  WindowManagerShortcutTable,
} from "@/systems/settings";

/**
 * The loaded page. Keyboard state is daemon-owned and applies live, so the
 * table, aliases and global hotkeys write through their own path and the page
 * has no save bar (US-022.AC-3).
 */
function ShortcutsSettingsView({
  clientId,
  focusCommandId,
  meta,
  section,
  workspaceId,
}: {
  clientId: string | undefined;
  focusCommandId?: string;
  meta: ReadonlyArray<{ key: string; content: ReactNode }>;
  section: WindowManagerSettingsSection;
  workspaceId: string;
}) {
  const { aliases, globalRecorder, recorder, bindingApply } = useWindowManagerKeyboardEditors(
    section,
    workspaceId,
    clientId
  );

  return (
    <SettingsPageFrame
      description="Changes apply as you make them."
      meta={meta}
      slug="shortcuts"
      width="wide"
    >
      <SettingsGroup
        action={<SettingsLiveChip />}
        bare
        help="Click a shortcut to change it. Only the ones you change are stored."
        title="Shortcuts"
      >
        <div className="overflow-hidden rounded-lg bg-card shadow-card">
          <WindowManagerShortcutTable
            aliases={aliases}
            apply={bindingApply}
            focusCommandId={focusCommandId}
            recorder={recorder}
            section={section}
          />
          <WindowManagerGlobalHotkeys recorder={globalRecorder} section={section} />
        </div>
      </SettingsGroup>
    </SettingsPageFrame>
  );
}

/** Every keyboard shortcut, its aliases, and the global hotkeys for the active workspace. */
export function ShortcutsSettingsPage({ focusCommandId }: { focusCommandId?: string }) {
  useSettingsTopbar();
  const data = useShortcutsSettingsData();

  if (data.isPending) return <SettingsPageState slug="shortcuts" state="loading" />;
  if (data.error !== null) {
    return (
      <SettingsPageState error={data.error} onRetry={data.retry} slug="shortcuts" state="error" />
    );
  }
  if (data.keyboard === null) return <SettingsPageState slug="shortcuts" state="loading" />;

  const meta = [
    data.workspaceName ? { key: "workspace", content: <span>{data.workspaceName}</span> } : null,
    data.keyboard.scope === "workspace"
      ? { key: "scope", content: <span>Saved for this project</span> }
      : null,
  ].filter((entry): entry is NonNullable<typeof entry> => entry !== null);

  return (
    <ShortcutsSettingsView
      clientId={data.clientId}
      focusCommandId={focusCommandId}
      meta={meta}
      section={data.keyboard}
      workspaceId={data.workspaceId}
    />
  );
}
