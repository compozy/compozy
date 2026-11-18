import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import type { FileViewerMode, TextPreviewKind } from "../lib/file-preview";

export const FILE_VIEWER_MODE_STORAGE_KEY = "compozy:files:viewer-mode:v1";

/** A file with a rendered form opens in Preview until the user picks another mode for its kind. */
export const DEFAULT_FILE_VIEWER_MODE: FileViewerMode = "preview";

const MODES: ReadonlySet<string> = new Set<FileViewerMode>(["source", "preview", "split"]);

interface FileViewerModeContext {
  byKind: Partial<Record<TextPreviewKind, FileViewerMode>>;
}

function mergeModes(
  persisted: Partial<FileViewerModeContext>,
  current: FileViewerModeContext
): FileViewerModeContext {
  const byKind = { ...current.byKind };
  const stored = persisted.byKind;
  if (typeof stored === "object" && stored !== null && !Array.isArray(stored)) {
    for (const [kind, mode] of Object.entries(stored)) {
      if (typeof mode === "string" && MODES.has(mode)) {
        byKind[kind as TextPreviewKind] = mode as FileViewerMode;
      }
    }
  }
  return { byKind };
}

const fileViewerModeLogic = createStoreLogic({
  context: (): FileViewerModeContext => ({ byKind: {} }),
  on: {
    modeChosen: (context, event: { kind: TextPreviewKind; mode: FileViewerMode }) => ({
      byKind: { ...context.byKind, [event.kind]: event.mode },
    }),
  },
});

export const fileViewerModeStore = fileViewerModeLogic
  .createStore()
  .with(persist({ name: FILE_VIEWER_MODE_STORAGE_KEY, merge: mergeModes }));

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === FILE_VIEWER_MODE_STORAGE_KEY) void rehydrateStore(fileViewerModeStore);
  });
}

/** The remembered Source / Preview / Split mode for one preview kind, and its setter. */
export function useFileViewerMode(
  kind: TextPreviewKind | null
): [FileViewerMode, (mode: FileViewerMode) => void] {
  const mode = useSelector(fileViewerModeStore, snapshot =>
    kind ? (snapshot.context.byKind[kind] ?? DEFAULT_FILE_VIEWER_MODE) : "source"
  );
  const setMode = (next: FileViewerMode) => {
    if (kind) fileViewerModeStore.trigger.modeChosen({ kind, mode: next });
  };
  return [mode, setMode];
}
