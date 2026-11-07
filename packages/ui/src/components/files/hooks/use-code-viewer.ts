"use client";

import type { CodeViewHandle, CodeViewItem, FileContents } from "@pierre/diffs/react";
import * as React from "react";

import { useResolvedCodeTheme } from "../../custom/hooks/use-code-block";
import { contentVersion, countLines, resolveTargetLine } from "../code-viewer-lines";
import type { ResolvedTargetLine } from "../code-viewer-lines";
import { PIERRE_CODE_THEMES } from "../pierre-theme";
import { useCodeViewerFind, type CodeViewerMatch } from "./use-code-viewer-find";

export interface UseCodeViewerOptions {
  cacheKey?: string;
  findMatches?: readonly CodeViewerMatch[];
  activeFindMatch?: number | null;
  language?: string;
  path: string;
  targetLine?: number | null;
  text: string;
  wrap: boolean;
}

const NO_MATCHES: readonly CodeViewerMatch[] = [];

export function useCodeViewer({
  cacheKey,
  findMatches = NO_MATCHES,
  activeFindMatch = null,
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

  const find = useCodeViewerFind(findMatches, activeFindMatch);
  const activeFind = activeFindMatch === null ? undefined : findMatches[activeFindMatch];
  const activeFindLine = activeFind?.line ?? null;
  // The current match scrolls into view even when its line was never rendered (US-015.AC-1).
  React.useEffect(() => {
    if (activeFindLine === null) return;
    handleRef.current?.scrollTo({
      type: "line",
      id: path,
      lineNumber: activeFindLine,
      align: "center",
    });
  }, [path, activeFindLine, activeFindMatch, version]);

  const { scheduleSync, unsafeCSS } = find;
  const options = React.useMemo(
    () => ({
      theme: PIERRE_CODE_THEMES,
      themeType,
      disableFileHeader: true,
      overflow: wrap ? ("wrap" as const) : ("scroll" as const),
      onPostRender: scheduleSync,
      unsafeCSS,
    }),
    [themeType, wrap, scheduleSync, unsafeCSS]
  );

  return {
    activeFindLine,
    containerRef: find.containerRef,
    handleRef,
    items,
    lineCount,
    onScroll: scheduleSync,
    options,
    selectedLines,
    target,
  };
}
