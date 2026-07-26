import css from "@shikijs/langs/css";
import diff from "@shikijs/langs/diff";
import go from "@shikijs/langs/go";
import html from "@shikijs/langs/html";
import javascript from "@shikijs/langs/javascript";
import json from "@shikijs/langs/json";
import jsx from "@shikijs/langs/jsx";
import markdown from "@shikijs/langs/markdown";
import python from "@shikijs/langs/python";
import sh from "@shikijs/langs/sh";
import sql from "@shikijs/langs/sql";
import toml from "@shikijs/langs/toml";
import tsx from "@shikijs/langs/tsx";
import typescript from "@shikijs/langs/typescript";
import yaml from "@shikijs/langs/yaml";
import vitesseDark from "@shikijs/themes/vitesse-dark";
import vitesseLight from "@shikijs/themes/vitesse-light";
import { createHighlighterCore, type ThemedToken } from "shiki/core";
import { createJavaScriptRegexEngine } from "shiki/engine/javascript";

import {
  normalizeAghCodeLanguage,
  resolveAghCodeThemeName,
  type AghCodeLanguage,
  type AghCodeThemeName,
  type CodeBlockResolvedTheme,
} from "./code-theme";

const FONT_STYLE_ITALIC = 1;
const FONT_STYLE_BOLD = 2;
const FONT_STYLE_UNDERLINE = 4;
const FONT_STYLE_STRIKETHROUGH = 8;

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
  language: AghCodeLanguage;
  lines: HighlightedCodeLine[];
  themeName: AghCodeThemeName;
}

export interface HighlightAghCodeOptions {
  code: string;
  language?: string | null;
  theme: CodeBlockResolvedTheme;
}

const highlighterPromise = createHighlighterCore({
  engine: createJavaScriptRegexEngine(),
  themes: [vitesseLight, vitesseDark],
  langs: [
    sh,
    css,
    diff,
    go,
    html,
    javascript,
    json,
    jsx,
    markdown,
    python,
    sql,
    toml,
    tsx,
    typescript,
    yaml,
  ],
  warnings: false,
});

export async function highlightAghCode({
  code,
  language,
  theme,
}: HighlightAghCodeOptions): Promise<HighlightedCodeResult | null> {
  const normalizedLanguage = normalizeAghCodeLanguage(language);
  if (!normalizedLanguage) return null;

  const highlighter = await highlighterPromise;
  const themeName = resolveAghCodeThemeName(theme);
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
