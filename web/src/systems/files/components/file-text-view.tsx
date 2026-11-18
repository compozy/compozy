import type { ReactNode } from "react";

import {
  CodeViewer,
  FileImagePreview,
  ResizableHandle,
  ResizablePanel,
  ResizablePanelGroup,
} from "@compozy/ui";

import type { FileContentResponse, FilesRootRef } from "../adapters/files-api";
import { useFileTextView } from "../hooks/use-file-text-view";
import type { FileViewerMode, TextPreviewKind } from "../lib/file-preview";
import type { ViewerFindSeed } from "../lib/file-viewer-target";
import { FileTextPreview } from "./file-text-preview";
import { ViewerFindBar } from "./viewer-find-bar";

export interface FileTextViewProps {
  root: FilesRootRef;
  path: string;
  /** The source shown in Source mode and searched by Find. */
  text: string;
  content: FileContentResponse;
  /** The file's rendered form; `null` when it has none or it is truncated (Source only). */
  previewKind: TextPreviewKind | null;
  line?: number;
  findSeed?: ViewerFindSeed;
  /** Builds the header for the current mode; the toggle shows when it receives a mode. */
  header: (mode?: FileViewerMode, onModeChange?: (mode: FileViewerMode) => void) => ReactNode;
  /** Notices under the header (stale, truncated); Find's bar follows them in Source. */
  notices: ReactNode;
  onOpenFile?: (path: string) => void;
}

function basename(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1) || path;
}

function PreviewBody({
  kind,
  root,
  path,
  text,
  content,
  onOpenFile,
}: {
  kind: TextPreviewKind;
  root: FilesRootRef;
  path: string;
  text: string;
  content: FileContentResponse;
  onOpenFile?: (path: string) => void;
}) {
  if (kind === "svg") {
    return (
      <FileImagePreview
        data={content.image_base64 ?? ""}
        mimeType="image/svg+xml"
        alt={basename(path)}
        data-testid="file-viewer-image"
      />
    );
  }
  return (
    <FileTextPreview kind={kind} root={root} path={path} text={text} onOpenFile={onOpenFile} />
  );
}

/**
 * An open text file (or SVG): Source in the virtualized code viewer, Preview in the format's
 * renderer, or both side by side. A file opened at a line or a Find seed starts in Source so the
 * target stays visible; picking a mode remembers it for the file's kind.
 */
export function FileTextView({
  root,
  path,
  text,
  content,
  previewKind,
  line,
  findSeed,
  header,
  notices,
  onOpenFile,
}: FileTextViewProps) {
  const { mode, chooseMode, find, closeFind, viewerRef, onKeyDown } = useFileTextView({
    text,
    previewKind,
    line,
    findSeed,
  });
  const findBar = find.open ? (
    <ViewerFindBar find={find} truncated={content.truncated} onClose={closeFind} />
  ) : null;
  const modeHeader = previewKind ? header(mode, chooseMode) : header();
  const source = (withHeader: boolean) => (
    <CodeViewer
      path={path}
      text={text}
      targetLine={line ?? null}
      cacheKey={content.revision ?? `${content.size}`}
      header={withHeader ? modeHeader : undefined}
      notice={
        <>
          {withHeader ? notices : null}
          {findBar}
        </>
      }
      findMatches={find.matches}
      activeFindMatch={find.active}
      pastEndNotice={(requested, last) =>
        `Line ${requested.toLocaleString("en-US")} is past the end (${last.toLocaleString("en-US")} lines)`
      }
      data-testid="file-viewer-text"
    />
  );
  const preview = previewKind ? (
    <div className="flex h-full min-h-0 flex-col" data-testid="file-viewer-preview">
      <PreviewBody
        kind={previewKind}
        root={root}
        path={path}
        text={text}
        content={content}
        onOpenFile={onOpenFile}
      />
    </div>
  ) : null;
  return (
    // The viewer is a labelled region that takes focus programmatically and owns its
    // shortcuts (Cmd/Ctrl+F, Cmd/Ctrl+Shift+V, Escape); it is not itself a control.
    <div
      ref={viewerRef}
      role="region"
      aria-label={`Contents of ${path}`}
      tabIndex={-1}
      onKeyDown={onKeyDown}
      className="flex h-full min-h-0 flex-col outline-none"
      data-testid="file-viewer-focus"
      data-mode={mode}
    >
      {mode === "source" ? (
        source(true)
      ) : (
        <>
          {modeHeader}
          {notices}
          {mode === "preview" ? (
            <div className="flex min-h-0 flex-1 flex-col">{preview}</div>
          ) : (
            <ResizablePanelGroup
              orientation="horizontal"
              className="min-h-0 flex-1"
              data-testid="file-viewer-split"
            >
              <ResizablePanel minSize={200} style={{ overflow: "hidden" }}>
                {source(false)}
              </ResizablePanel>
              <ResizableHandle aria-label="Resize source and preview" />
              <ResizablePanel minSize={200} style={{ overflow: "hidden" }}>
                {preview}
              </ResizablePanel>
            </ResizablePanelGroup>
          )}
        </>
      )}
    </div>
  );
}
