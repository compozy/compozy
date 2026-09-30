import { createStoreLogic } from "@xstate/store";

import {
  getSystemPrefersDark,
  readThemePreference,
  resolveTheme,
  writeThemePreference,
} from "../lib/theme-preference";
import type { ResolvedTheme, ThemePreference } from "../types";

interface ThemePreferenceContext {
  preference: ThemePreference;
  systemPrefersDark: boolean;
}

export const themePreferenceLogic = createStoreLogic({
  context: (input: ThemePreferenceContext): ThemePreferenceContext => ({ ...input }),
  on: {
    /** An explicit choice (Appearance pane): persisted. */
    preferenceSet: (context, event: { preference: ThemePreference }, enqueue) => {
      if (context.preference === event.preference) return context;
      enqueue.effect(() => writeThemePreference(event.preference));
      return { ...context, preference: event.preference };
    },
    /** Rail-foot toggle: flips the painted theme by storing an explicit light/dark (D4). */
    resolvedToggled: (context, _event, enqueue) => {
      const next: ThemePreference = selectResolvedTheme(context) === "dark" ? "light" : "dark";
      enqueue.effect(() => writeThemePreference(next));
      return { ...context, preference: next };
    },
    /** Another tab wrote the key: adopt it without writing it back. */
    preferenceSynced: (context, event: { preference: ThemePreference }) =>
      context.preference === event.preference
        ? context
        : { ...context, preference: event.preference },
    systemSchemeChanged: (context, event: { prefersDark: boolean }) =>
      context.systemPrefersDark === event.prefersDark
        ? context
        : { ...context, systemPrefersDark: event.prefersDark },
  },
});

export type ThemePreferenceStore = ReturnType<typeof themePreferenceLogic.createStore>;

export function selectResolvedTheme(context: ThemePreferenceContext): ResolvedTheme {
  return resolveTheme(context.preference, context.systemPrefersDark);
}

/**
 * One process-wide store for this browser's theme (D3): read once from local
 * storage, written on every explicit change. Never a daemon key by decision.
 */
export const themePreferenceStore: ThemePreferenceStore = themePreferenceLogic.createStore({
  preference: readThemePreference(),
  systemPrefersDark: getSystemPrefersDark(),
});
