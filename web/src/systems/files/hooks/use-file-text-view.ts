import type { KeyboardEvent } from "react";
import { useRef, useState } from "react";

import type { FileViewerMode, TextPreviewKind } from "../lib/file-preview";
import type { ViewerFindSeed } from "../lib/file-viewer-target";
import { isFindChord, isPreviewChord } from "../lib/files-shortcuts";
import { useFileViewerMode } from "./use-file-viewer-mode";
import { useViewerFind } from "./use-viewer-find";

/**
 * The open text file's view state: its mode (remembered per preview kind; a file opened at a line
 * or a Find seed starts in Source so the target stays visible), Find over the source, and the
 * viewer-scoped chords.
 */
export function useFileTextView({
  text,
  previewKind,
  line,
  findSeed,
}: {
  text: string;
  previewKind: TextPreviewKind | null;
  line?: number;
  findSeed?: ViewerFindSeed;
}) {
  const [rememberedMode, rememberMode] = useFileViewerMode(previewKind);
  const [override, setOverride] = useState<FileViewerMode | null>(
    previewKind && (line !== undefined || findSeed !== undefined) ? "source" : null
  );
  const mode: FileViewerMode = previewKind ? (override ?? rememberedMode) : "source";
  const find = useViewerFind(text, findSeed, line);
  const viewerRef = useRef<HTMLDivElement | null>(null);
  const chooseMode = (next: FileViewerMode) => {
    setOverride(null);
    rememberMode(next);
  };
  const closeFind = () => {
    find.close();
    viewerRef.current?.focus({ preventScroll: true });
  };
  // Viewer-scoped: these chords only take over while focus is inside the viewer, so the
  // browser's find runs everywhere else (US-015.EC-3).
  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (isFindChord(event)) {
      event.preventDefault();
      // Find searches the source, so a rendered-only view shows it first.
      if (mode === "preview") chooseMode("source");
      find.show();
    } else if (previewKind && isPreviewChord(event)) {
      event.preventDefault();
      chooseMode(mode === "preview" ? "source" : "preview");
    } else if (event.key === "Escape" && find.open) {
      event.preventDefault();
      closeFind();
    }
  };
  return { mode, chooseMode, find, closeFind, viewerRef, onKeyDown };
}
