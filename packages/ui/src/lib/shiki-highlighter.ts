import vitesseDark from "@shikijs/themes/vitesse-dark";
import vitesseLight from "@shikijs/themes/vitesse-light";
import {
  createHighlighterCore,
  type HighlighterCore,
  type LanguageRegistration,
  type ThemedToken,
} from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";

import {
  normalizeCompozyCodeLanguage,
  resolveCompozyCodeThemeName,
  type CompozyCodeLanguage,
  type CompozyCodeThemeName,
  type CodeBlockResolvedTheme,
} from "./code-theme";

const FONT_STYLE_ITALIC = 1;
const FONT_STYLE_BOLD = 2;
const FONT_STYLE_UNDERLINE = 4;
const FONT_STYLE_STRIKETHROUGH = 8;

type LanguageModule = { default: LanguageRegistration[] };
type LanguageLoader = () => Promise<LanguageModule>;

const languageLoaders: Record<CompozyCodeLanguage, LanguageLoader> = {
  bash: () => import("@shikijs/langs/sh"),
  css: () => import("@shikijs/langs/css"),
  diff: () => import("@shikijs/langs/diff"),
  go: () => import("@shikijs/langs/go"),
  html: () => import("@shikijs/langs/html"),
  javascript: () => import("@shikijs/langs/javascript"),
  json: () => import("@shikijs/langs/json"),
  jsx: () => import("@shikijs/langs/jsx"),
  markdown: () => import("@shikijs/langs/markdown"),
  python: () => import("@shikijs/langs/python"),
  sql: () => import("@shikijs/langs/sql"),
  toml: () => import("@shikijs/langs/toml"),
  tsx: () => import("@shikijs/langs/tsx"),
  typescript: () => import("@shikijs/langs/typescript"),
  yaml: () => import("@shikijs/langs/yaml"),
};

const loadedLanguages = new Set<CompozyCodeLanguage>();
const languageLoads = new Map<CompozyCodeLanguage, Promise<void>>();

export interface HighlightedCodeToken {
  color?: string;
  content: string;
  fontStyle?: "italic";
  fontWeight?: "bold";
  textDecorationLine?: "line-through" | "underline" | "underline line-through";
}

export interface HighlightedCodeLine {
  lineNumber: number;
  tokens: HighlightedCodeToken[];
}

export interface HighlightedCodeResult {
  language: CompozyCodeLanguage;
  lines: HighlightedCodeLine[];
  themeName: CompozyCodeThemeName;
}

export interface HighlightCompozyCodeOptions {
  code: string;
  language?: string | null;
  theme: CodeBlockResolvedTheme;
}

const highlighterPromise = createHighlighterCore({
  engine: createJavaScriptRegexEngine(),
  themes: [vitesseLight, vitesseDark],
  langs: [],
  warnings: false,
});

export async function highlightCompozyCode({
  code,
  language,
  theme,
}: HighlightCompozyCodeOptions): Promise<HighlightedCodeResult | null> {
  const normalizedLanguage = normalizeCompozyCodeLanguage(language);
  if (!normalizedLanguage) return null;

  const highlighter = await highlighterPromise;
  await ensureLanguage(highlighter, normalizedLanguage);
  const themeName = resolveCompozyCodeThemeName(theme);
  const result = highlighter.codeToTokens(code, {
    lang: normalizedLanguage,
    theme: themeName,
    tokenizeTimeLimit: 1_000,
    tokenizeMaxLineLength: 20_000,
  });

  return {
    language: normalizedLanguage,
    themeName,
    lines: result.tokens.map((tokens, index) => ({
      lineNumber: index + 1,
      tokens: tokens.map(toHighlightedCodeToken),
    })),
  };
}

async function ensureLanguage(
  highlighter: HighlighterCore,
  language: CompozyCodeLanguage
): Promise<void> {
  if (loadedLanguages.has(language)) return;

  const activeLoad = languageLoads.get(language);
  if (activeLoad) return activeLoad;

  const load = languageLoaders[language]()
    .then(module => highlighter.loadLanguage(module.default))
    .then(() => {
      loadedLanguages.add(language);
    })
    .finally(() => {
      languageLoads.delete(language);
    });
  languageLoads.set(language, load);
  return load;
}

function toHighlightedCodeToken(token: ThemedToken): HighlightedCodeToken {
  const decorations: Array<"line-through" | "underline"> = [];
  const fontStyle = token.fontStyle ?? 0;
  if (fontStyle & FONT_STYLE_UNDERLINE) decorations.push("underline");
  if (fontStyle & FONT_STYLE_STRIKETHROUGH) decorations.push("line-through");

  return {
    content: token.content,
    color: token.color,
    fontStyle: fontStyle & FONT_STYLE_ITALIC ? "italic" : undefined,
    fontWeight: fontStyle & FONT_STYLE_BOLD ? "bold" : undefined,
    textDecorationLine: toTextDecorationLine(decorations),
  };
}

function toTextDecorationLine(
  decorations: Array<"line-through" | "underline">
): HighlightedCodeToken["textDecorationLine"] {
  const hasUnderline = decorations.includes("underline");
  const hasLineThrough = decorations.includes("line-through");
  if (hasUnderline && hasLineThrough) return "underline line-through";
  if (hasUnderline) return "underline";
  if (hasLineThrough) return "line-through";
  return undefined;
}
