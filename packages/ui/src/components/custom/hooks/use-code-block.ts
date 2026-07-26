import * as React from "react";

import {
  AGH_CODE_DEFAULT_THEME,
  normalizeAghCodeLanguage,
  resolveAghCodeThemeName,
  type CodeBlockResolvedTheme,
  type CodeBlockThemeMode,
} from "../../../lib/code-theme";
import { highlightAghCode, type HighlightedCodeLine } from "../../../lib/shiki-highlighter";

export type CodeBlockHighlightState = "plain" | "loading" | "highlighted" | "failed";

interface UseCodeBlockOptions {
  code: string;
  highlightLines?: readonly number[];
  language?: string;
  themeMode: CodeBlockThemeMode;
  truncateLines?: number;
}

export function useCodeBlock({
  code,
  highlightLines,
  language,
  themeMode,
  truncateLines,
}: UseCodeBlockOptions) {
  const resolvedTheme = useResolvedCodeTheme(themeMode);
  const resolvedThemeName = resolveAghCodeThemeName(resolvedTheme);
  const normalizedLanguage = normalizeAghCodeLanguage(language);
  const [highlightedCode, setHighlightedCode] = React.useState<HighlightedCodeLine[] | null>(null);
  const [highlightState, setHighlightState] = React.useState<CodeBlockHighlightState>(
    normalizedLanguage ? "loading" : "plain"
  );

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

  React.useEffect(() => {
    let cancelled = false;

    if (!normalizedLanguage) {
      setHighlightedCode(null);
      setHighlightState("plain");
      return () => {
        cancelled = true;
      };
    }

    setHighlightState("loading");
    setHighlightedCode(null);

    void highlightAghCode({ code, language: normalizedLanguage, theme: resolvedTheme })
      .then(result => {
        if (cancelled) return;
        if (!result) {
          setHighlightedCode(null);
          setHighlightState("plain");
          return;
        }
        setHighlightedCode(result.lines);
        setHighlightState("highlighted");
      })
      .catch((error: unknown) => {
        if (cancelled) return;
        console.error("Failed to highlight code block", error);
        setHighlightedCode(null);
        setHighlightState("failed");
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
  const [resolvedTheme, setResolvedTheme] = React.useState<CodeBlockResolvedTheme>(() =>
    themeMode === "auto" ? AGH_CODE_DEFAULT_THEME : themeMode
  );

  React.useEffect(() => {
    if (themeMode !== "auto") {
      setResolvedTheme(themeMode);
      return;
    }

    const update = () => setResolvedTheme(resolveAutoCodeTheme());
    update();

    if (typeof MutationObserver === "undefined" || typeof document === "undefined") return;

    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, { attributes: true, attributeFilter: ["class"] });
    if (document.body) {
      observer.observe(document.body, { attributes: true, attributeFilter: ["class"] });
    }

    return () => observer.disconnect();
  }, [themeMode]);

  return resolvedTheme;
}

function resolveAutoCodeTheme(): CodeBlockResolvedTheme {
  if (typeof document === "undefined") return AGH_CODE_DEFAULT_THEME;
  const root = document.documentElement;
  const body = document.body;
  return root.classList.contains("dark") || body?.classList.contains("dark") ? "dark" : "light";
}
