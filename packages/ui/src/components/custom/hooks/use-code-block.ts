import * as React from "react";

import {
  COMPOZY_CODE_DEFAULT_THEME,
  normalizeCompozyCodeLanguage,
  resolveCompozyCodeThemeName,
  type CodeBlockResolvedTheme,
  type CodeBlockThemeMode,
} from "../../../lib/code-theme";
import type { HighlightedCodeLine } from "../../../lib/shiki-highlighter";

export type CodeBlockHighlightState = "plain" | "loading" | "highlighted" | "failed";

interface UseCodeBlockOptions {
  code: string;
  highlightLines?: readonly number[];
  language?: string;
  themeMode: CodeBlockThemeMode;
  truncateLines?: number;
}

interface CodeHighlightResult {
  code: string;
  language: string;
  lines: HighlightedCodeLine[] | null;
  state: Exclude<CodeBlockHighlightState, "loading">;
  theme: CodeBlockResolvedTheme;
}

export function useCodeBlock({
  code,
  highlightLines,
  language,
  themeMode,
  truncateLines,
}: UseCodeBlockOptions) {
  const resolvedTheme = useResolvedCodeTheme(themeMode);
  const resolvedThemeName = resolveCompozyCodeThemeName(resolvedTheme);
  const normalizedLanguage = normalizeCompozyCodeLanguage(language);
  const [highlightResult, setHighlightResult] = React.useState<CodeHighlightResult | null>(null);

  const lines = code.split("\n");
  const seenLines = new Map<string, number>();
  const displayLines = lines.map((line, index) => {
    const count = seenLines.get(line) ?? 0;
    seenLines.set(line, count + 1);
    return { id: `${index + 1}:${line || "blank"}-${count}`, line, lineNumber: index + 1 };
  });
  const highlightedLineNumbers = new Set(
    highlightLines?.filter(line => Number.isInteger(line) && line > 0) ?? []
  );
  const clampedLines =
    typeof truncateLines === "number" && Number.isFinite(truncateLines) && truncateLines > 0
      ? Math.floor(truncateLines)
      : undefined;
  const highlightResultMatches =
    highlightResult?.code === code &&
    highlightResult.language === normalizedLanguage &&
    highlightResult.theme === resolvedTheme;
  const highlightedCode = highlightResultMatches ? highlightResult.lines : null;
  const highlightState: CodeBlockHighlightState = !normalizedLanguage
    ? "plain"
    : highlightResultMatches
      ? highlightResult.state
      : "loading";

  React.useEffect(() => {
    if (!normalizedLanguage) return undefined;
    let cancelled = false;

    void import("../../../lib/shiki-highlighter")
      .then(({ highlightCompozyCode }) =>
        highlightCompozyCode({ code, language: normalizedLanguage, theme: resolvedTheme })
      )
      .then(result => {
        if (cancelled) return;
        if (!result) {
          setHighlightResult({
            code,
            language: normalizedLanguage,
            lines: null,
            state: "plain",
            theme: resolvedTheme,
          });
          return;
        }
        setHighlightResult({
          code,
          language: normalizedLanguage,
          lines: result.lines,
          state: "highlighted",
          theme: resolvedTheme,
        });
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        console.error("Failed to highlight code block", error);
        setHighlightResult({
          code,
          language: normalizedLanguage,
          lines: null,
          state: "failed",
          theme: resolvedTheme,
        });
      });

    return () => {
      cancelled = true;
    };
  }, [code, normalizedLanguage, resolvedTheme]);

  return {
    clampedLines,
    displayLines,
    highlightedCode,
    highlightedLineNumbers,
    highlightState,
    normalizedLanguage,
    resolvedThemeName,
  };
}

function useResolvedCodeTheme(themeMode: CodeBlockThemeMode): CodeBlockResolvedTheme {
  const subscribe = (onStoreChange: () => void) => {
    if (themeMode !== "auto") return () => {};
    return subscribeToCodeTheme(onStoreChange);
  };
  const getSnapshot = () => (themeMode === "auto" ? resolveAutoCodeTheme() : themeMode);
  const getServerSnapshot = () => (themeMode === "auto" ? COMPOZY_CODE_DEFAULT_THEME : themeMode);

  return React.useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot);
}

function subscribeToCodeTheme(onStoreChange: () => void): () => void {
  if (typeof MutationObserver === "undefined" || typeof document === "undefined") return () => {};

  const observer = new MutationObserver(onStoreChange);
  observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
  if (document.body) {
    observer.observe(document.body, { attributes: true, attributeFilter: ["class"] });
  }
  return () => observer.disconnect();
}

function resolveAutoCodeTheme(): CodeBlockResolvedTheme {
  if (typeof document === "undefined") return COMPOZY_CODE_DEFAULT_THEME;
  const root = document.documentElement;
  const body = document.body;
  return root.classList.contains("dark") || body?.classList.contains("dark") ? "dark" : "light";
}
