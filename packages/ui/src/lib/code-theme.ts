const AGH_CODE_SUPPORTED_LANGUAGES = [
  "bash",
  "css",
  "diff",
  "go",
  "html",
  "javascript",
  "json",
  "jsx",
  "markdown",
  "python",
  "sql",
  "toml",
  "tsx",
  "typescript",
  "yaml",
] as const;

export type AghCodeLanguage = (typeof AGH_CODE_SUPPORTED_LANGUAGES)[number];

const AGH_CODE_LANGUAGE_ALIASES: Record<string, AghCodeLanguage | ""> = {
  cjs: "javascript",
  js: "javascript",
  mjs: "javascript",
  md: "markdown",
  plain: "",
  plaintext: "",
  shell: "bash",
  sh: "bash",
  text: "",
  ts: "typescript",
  txt: "",
  yml: "yaml",
  zsh: "bash",
};

const AGH_CODE_LANGUAGE_SET = new Set<string>(AGH_CODE_SUPPORTED_LANGUAGES);

export const AGH_CODE_THEMES = {
  light: "vitesse-light",
  dark: "vitesse-dark",
} as const;

export const AGH_CODE_DEFAULT_THEME = "dark";

export type AghCodeThemeName = (typeof AGH_CODE_THEMES)[keyof typeof AGH_CODE_THEMES];
export type CodeBlockResolvedTheme = keyof typeof AGH_CODE_THEMES;
export type CodeBlockThemeMode = CodeBlockResolvedTheme | "auto";

export function normalizeAghCodeLanguage(language?: string | null): AghCodeLanguage | null {
  const rawLanguage =
    language
      ?.trim()
      .toLowerCase()
      .replace(/^language-/, "") ?? "";
  if (!rawLanguage) return null;

  const aliasedLanguage = AGH_CODE_LANGUAGE_ALIASES[rawLanguage];
  if (aliasedLanguage !== undefined) {
    return aliasedLanguage === "" ? null : aliasedLanguage;
  }

  return AGH_CODE_LANGUAGE_SET.has(rawLanguage) ? (rawLanguage as AghCodeLanguage) : null;
}

export function resolveAghCodeThemeName(theme: CodeBlockResolvedTheme): AghCodeThemeName {
  return AGH_CODE_THEMES[theme];
}

export { AGH_CODE_SUPPORTED_LANGUAGES };
