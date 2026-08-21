import type { WindowManagerSettingsSection } from "@/systems/os";

import { useGlobalShortcutRecorder } from "./use-global-shortcut-recorder";
import { useWindowManagerAliasEditor } from "./use-window-manager-alias-editor";
import { useWindowManagerBindingMutations } from "./use-window-manager-binding-mutations";
import { useWindowManagerShortcutRecorder } from "./use-window-manager-shortcut-recorder";

export function useWindowManagerKeyboardEditors(
  section: WindowManagerSettingsSection,
  workspaceId: string,
  clientId: string | undefined
) {
  const bindings = useWindowManagerBindingMutations(
    workspaceId === "" ? null : workspaceId,
    clientId
  );
  const recorder = useWindowManagerShortcutRecorder(section, bindings);
  const globalRecorder = useGlobalShortcutRecorder(section, bindings);
  const aliases = useWindowManagerAliasEditor(
    section,
    bindings,
    commandId => section.commands.find(command => command.id === commandId)?.title ?? commandId
  );
  return { aliases, globalRecorder, recorder };
}
