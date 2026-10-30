import { useEffect } from "react";

import { applyTheme } from "@/systems/theme";

import { prototypeParams } from "#gallery/contract";
import { defaultTheme, themeOf, type Theme } from "#gallery/shell/view";
import { useSearch } from "#gallery/use-search";

/** The theme the URL names (`?theme=light|dark`), dark by default like the app. */
export function useTheme(): Theme {
  const search = useSearch();
  return themeOf(new URLSearchParams(search).get(prototypeParams.theme)) ?? defaultTheme;
}

/** Paints the document in the URL theme through the app's only theme writer (`data-theme` + `.dark`). */
export function useApplyTheme(theme: Theme) {
  useEffect(() => {
    applyTheme(theme);
  }, [theme]);
}
