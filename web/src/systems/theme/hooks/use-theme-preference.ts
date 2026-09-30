import { useSelector } from "@xstate/store-react";

import { selectResolvedTheme, themePreferenceStore } from "../stores/theme-preference-store";
import type { ResolvedTheme, ThemePreference } from "../types";

export interface ThemePreferenceHandle {
  /** The stored choice: light, dark, or system. */
  preference: ThemePreference;
  /** The painted theme; consumers needing a JS theme value read this. */
  resolvedTheme: ResolvedTheme;
  setPreference: (preference: ThemePreference) => void;
  /** Flips the painted theme, storing an explicit light/dark (rail-foot toggle). */
  toggleResolved: () => void;
}

export function useThemePreference(): ThemePreferenceHandle {
  const preference = useSelector(themePreferenceStore, snapshot => snapshot.context.preference);
  const resolvedTheme = useSelector(themePreferenceStore, snapshot =>
    selectResolvedTheme(snapshot.context)
  );
  return {
    preference,
    resolvedTheme,
    setPreference: next => themePreferenceStore.trigger.preferenceSet({ preference: next }),
    toggleResolved: () => themePreferenceStore.trigger.resolvedToggled(),
  };
}
