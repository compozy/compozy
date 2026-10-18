import {
  selectResolvedTheme,
  themePreferenceStore,
  type ThemePreferenceStore,
} from "../stores/theme-preference-store";
import { applyTheme } from "./apply-theme";
import {
  THEME_STORAGE_KEY,
  parseThemePreference,
  subscribeSystemColorScheme,
} from "./theme-preference";
import type { ThemePreference } from "../types";

/**
 * Hands the preference to the Electron shell, which drives native chrome,
 * window backgrounds and the next launch's first paint from it. A plain
 * browser (or an older shell) has no `theme` bridge: nothing to do.
 */
function reportToDesktopShell(preference: ThemePreference): void {
  window.compozyShell?.theme?.set(preference).catch((error: unknown) => {
    console.warn("Failed to report the theme to the desktop shell", error);
  });
}

/**
 * Keeps the document painted with the resolved theme for the app's lifetime:
 * applies it now (re-asserting the boot script's pre-paint choice), on every
 * preference change, live when `system` follows an OS scheme change, and when
 * another tab writes the preference; reports every preference to the desktop
 * shell when there is one. Returns the teardown.
 */
export function installThemeRuntime(
  store: ThemePreferenceStore = themePreferenceStore,
  root: HTMLElement = document.documentElement
): () => void {
  const resolved = store.select(selectResolvedTheme);
  applyTheme(resolved.get(), root);
  const selection = resolved.subscribe(theme => applyTheme(theme, root));

  const preference = store.select(context => context.preference);
  reportToDesktopShell(preference.get());
  const reporting = preference.subscribe(reportToDesktopShell);

  const unsubscribeScheme = subscribeSystemColorScheme(prefersDark =>
    store.trigger.systemSchemeChanged({ prefersDark })
  );

  const handleStorage = (event: StorageEvent) => {
    if (event.key !== THEME_STORAGE_KEY) return;
    store.trigger.preferenceSynced({ preference: parseThemePreference(event.newValue) });
  };
  window.addEventListener("storage", handleStorage);

  return () => {
    selection.unsubscribe();
    reporting.unsubscribe();
    unsubscribeScheme();
    window.removeEventListener("storage", handleStorage);
  };
}
