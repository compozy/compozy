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

/**
 * Keeps the document painted with the resolved theme for the app's lifetime:
 * applies it now (re-asserting the boot script's pre-paint choice), on every
 * preference change, live when `system` follows an OS scheme change, and when
 * another tab writes the preference. Returns the teardown.
 */
export function installThemeRuntime(
  store: ThemePreferenceStore = themePreferenceStore,
  root: HTMLElement = document.documentElement
): () => void {
  const resolved = store.select(selectResolvedTheme);
  applyTheme(resolved.get(), root);
  const selection = resolved.subscribe(theme => applyTheme(theme, root));

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
    unsubscribeScheme();
    window.removeEventListener("storage", handleStorage);
  };
}
