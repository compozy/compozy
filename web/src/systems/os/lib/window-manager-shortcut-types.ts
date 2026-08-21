export type WindowManagerShortcutBinding = readonly string[];
export type WindowManagerShortcutMap = Readonly<Record<string, WindowManagerShortcutBinding>>;
export type WindowManagerGlobalShortcutMap = Readonly<Record<string, string>>;

export interface WindowManagerGlobalShortcutRegistration {
  commandId: string;
  intendedChord: string;
  activeChord: string | null;
  status: "pending" | "registered" | "failed_in_use" | "failed_permission" | "unsupported";
  reason: string | null;
  settingsUrl: string | null;
}
