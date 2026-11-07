"use client";

import type { CodeViewHandle, CodeViewItem, FileContents } from "@pierre/diffs/react";
import * as React from "react";

import { useResolvedCodeTheme } from "../../custom/hooks/use-code-block";
import { contentVersion, countLines, resolveTargetLine } from "../code-viewer-lines";
import type { ResolvedTargetLine } from "../code-viewer-lines";
import { PIERRE_CODE_THEMES } from "../pierre-theme";

export interface UseCodeViewerOptions {
  cacheKey?: string;
  language?: string;
  path: string;
  targetLine?: number | null;
  text: string;
  wrap: boolean;
}

export function useCodeViewer({
  cacheKey,
  language,
  path,
  targetLine,
  text,
  wrap,
}: UseCodeViewerOptions) {
  const handleRef = React.useRef<CodeViewHandle<undefined, undefined>>(null);
  const themeType = useResolvedCodeTheme("auto");

  // Pierre's VirtualizedFile asserts the rendered file is the exact object it
  // laid out (`VirtualizedFile.render` identity check, unchanged in 1.6.4), so
  // the file object only changes when its content does.
  const file = React.useMemo<FileContents>(
    () => ({
      name: path,
      contents: text,
      ...(language ? { lang: language } : {}),
      ...(cacheKey ? { cacheKey } : {}),
    }),
    [path, text, language, cacheKey]
  );
  const version = React.useMemo(() => contentVersion(text), [text]);
  const items = React.useMemo<CodeViewItem<undefined>[]>(
    () => [{ id: path, type: "file", file, version }],
    [path, file, version]
  );

  const lineCount = React.useMemo(() => countLines(text), [text]);
  const target: ResolvedTargetLine | null = resolveTargetLine(targetLine, lineCount);
  const targetLineNumber = target?.line ?? null;
  const selectedLines = React.useMemo(
    () =>
      targetLineNumber === null
        ? null
        : { id: path, range: { start: targetLineNumber, end: targetLineNumber } },
    [path, targetLineNumber]
  );

  React.useEffect(() => {
    if (targetLineNumber === null) return;
    handleRef.current?.scrollTo({
      type: "line",
      id: path,
      lineNumber: targetLineNumber,
      align: "center",
    });
  }, [path, targetLineNumber, version]);

  const options = React.useMemo(
    () => ({
      theme: PIERRE_CODE_THEMES,
      themeType,
      disableFileHeader: true,
      overflow: wrap ? ("wrap" as const) : ("scroll" as const),
    }),
    [themeType, wrap]
  );

  return { handleRef, items, lineCount, options, selectedLines, target };
}
