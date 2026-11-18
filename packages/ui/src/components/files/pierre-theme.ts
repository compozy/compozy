import { COMPOZY_CODE_THEMES } from "../../lib/code-theme";

/**
 * The Shiki pair every Pierre surface highlights with — the same pair
 * `CodeBlock` uses (`lib/code-theme.ts`). Each viewer picks a side through
 * `themeType`, resolved from the document theme.
 */
export const PIERRE_CODE_THEMES = {
  dark: COMPOZY_CODE_THEMES.dark,
  light: COMPOZY_CODE_THEMES.light,
} as const;
