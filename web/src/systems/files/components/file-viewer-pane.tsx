import { useQuery } from "@tanstack/react-query";
import type { KeyboardEvent } from "react";
import { useRef, useState } from "react";

import { CodeViewer, FileImagePreview } from "@compozy/ui";
import { Info } from "@compozy/ui/icons";

import type { FilesRootRef } from "../adapters/files-api";
import { useViewerFind } from "../hooks/use-viewer-find";
import { copyFilePath } from "../lib/copy-file-path";
import { FILE_TEXT_LIMIT, formatFileSize } from "../lib/format-file-size";
import { fileViewerState, type FileViewerState } from "../lib/file-viewer-state";
import type { FileViewerTarget, ViewerFindSeed } from "../lib/file-viewer-target";
import { isFindChord } from "../lib/files-shortcuts";
import { fileContentOptions } from "../lib/query-options";
import { FileViewerHeader } from "./file-viewer-header";
import {
  FileViewerBinaryState,
  FileViewerImageTooLargeState,
  FileViewerLoadingState,
  FileViewerMessageState,
  FileViewerNotFoundState,
  FileViewerOutsideState,
  FileViewerSelectState,
  FileViewerSensitiveState,
} from "./file-viewer-states";
import { OpenInButton } from "./open-in-button";
import { ViewerFindBar } from "./viewer-find-bar";

export interface FileViewerPaneProps {
  root: FilesRootRef;
  target: FileViewerTarget | null;
  /** Selects and scrolls the tree to the open file. */
  onRevealInTree?: (path: string) => void;
  /** Refetches the tree listings (the "File not found" recovery). */
  onRefreshTree?: () => void;
}

function basename(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1) || path;
}

function TruncationNotice({ size }: { size: number }) {
  return (
    <p
      role="status"
      data-testid="file-viewer-truncated"
      className="flex items-center gap-2 border-b border-line-soft px-3 py-1.5 text-xs text-muted-foreground"
    >
      <Info aria-hidden className="size-3.5 shrink-0" />
      Showing the first {formatFileSize(FILE_TEXT_LIMIT)} of {formatFileSize(size)}
    </p>
  );
}

/**
 * One open file: its content query, the sensitive gate, and the state to show. Keyed by the
 * target's nonce, so a reopened file starts gated again and old content never mixes with new.
 */
function OpenFileView({
  root,
  path,
  line,
  findSeed,
  onRevealInTree,
  onRefreshTree,
}: {
  root: FilesRootRef;
  path: string;
  line?: number;
  findSeed?: ViewerFindSeed;
} & Pick<FileViewerPaneProps, "onRevealInTree" | "onRefreshTree">) {
  const [revealed, setRevealed] = useState(false);
  const gated = useQuery(fileContentOptions(root, path, false));
  const reveal = useQuery({ ...fileContentOptions(root, path, true), enabled: revealed });
  const active = revealed ? reveal : gated;
  const state: FileViewerState = fileViewerState(active);
  const content = "content" in state ? state.content : undefined;
  const find = useViewerFind(state.kind === "text" ? state.text : "", findSeed, line);
  const viewerRef = useRef<HTMLDivElement | null>(null);
  const header = (
    <FileViewerHeader
      path={path}
      size={content?.size}
      refreshing={active.isFetching && !active.isPending}
      onCopyPath={() => copyFilePath(path)}
      onRevealInTree={onRevealInTree ? () => onRevealInTree(path) : undefined}
      onRefresh={() => void active.refetch()}
      openIn={<OpenInButton root={root} path={path} line={line} />}
    />
  );

  if (state.kind === "text") {
    const closeFind = () => {
      find.close();
      viewerRef.current?.focus({ preventScroll: true });
    };
    // Viewer-scoped: Cmd/Ctrl+F only takes over while focus is inside the viewer, so the
    // browser's find runs everywhere else (US-015.EC-3).
    const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
      if (isFindChord(event)) {
        event.preventDefault();
        find.show();
      } else if (event.key === "Escape" && find.open) {
        event.preventDefault();
        closeFind();
      }
    };
    const truncation = state.content.truncated ? (
      <TruncationNotice size={state.content.size} />
    ) : null;
    return (
      // The viewer is a labelled region that takes focus programmatically and owns its
      // shortcuts (Cmd/Ctrl+F, Escape); it is not itself a control.
      <div
        ref={viewerRef}
        role="region"
        aria-label={`Contents of ${path}`}
        tabIndex={-1}
        onKeyDown={onKeyDown}
        className="flex h-full min-h-0 flex-col outline-none"
        data-testid="file-viewer-focus"
      >
        <CodeViewer
          path={path}
          text={state.text}
          targetLine={line ?? null}
          cacheKey={state.content.revision ?? `${state.content.size}`}
          header={header}
          notice={
            <>
              {truncation}
              {find.open ? (
                <ViewerFindBar
                  find={find}
                  truncated={state.content.truncated}
                  onClose={closeFind}
                />
              ) : null}
            </>
          }
          findMatches={find.matches}
          activeFindMatch={find.active}
          pastEndNotice={(requested, last) =>
            `Line ${requested.toLocaleString("en-US")} is past the end (${last.toLocaleString("en-US")} lines)`
          }
          data-testid="file-viewer-text"
        />
      </div>
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      {header}
      <div className="flex min-h-0 flex-1 flex-col overflow-auto">
        {stateBody(state, {
          root,
          line,
          path,
          revealing: revealed && reveal.isPending,
          onReveal: () => setRevealed(true),
          onRetry: () => void active.refetch(),
          onRefreshTree,
        })}
      </div>
    </div>
  );
}

function stateBody(
  state: FileViewerState,
  props: {
    root: FilesRootRef;
    line?: number;
    path: string;
    revealing: boolean;
    onReveal: () => void;
    onRetry: () => void;
    onRefreshTree?: () => void;
  }
) {
  switch (state.kind) {
    case "loading":
      return <FileViewerLoadingState />;
    case "not_found":
      return <FileViewerNotFoundState path={props.path} onRefreshTree={props.onRefreshTree} />;
    case "outside":
      return (
        <FileViewerOutsideState
          path={props.path}
          openIn={<OpenInButton root={props.root} path={props.path} line={props.line} />}
        />
      );
    case "denied":
      return (
        <FileViewerMessageState
          testId="file-viewer-denied"
          title="This file can't be shown"
          description="It resolves into a protected location."
        />
      );
    case "not_a_file":
      return <FileViewerMessageState testId="file-viewer-not-a-file" title="Not a file" />;
    case "error":
      return (
        <FileViewerMessageState
          testId="file-viewer-error"
          title="Couldn't read this file"
          description={state.message}
          onRetry={props.onRetry}
        />
      );
    case "image":
      return (
        <FileImagePreview
          data={state.content.image_base64 ?? ""}
          mimeType={state.content.image_mime ?? ""}
          alt={basename(props.path)}
          meta={formatFileSize(state.content.size)}
          fallback={<FileViewerBinaryState size={state.content.size} />}
          data-testid="file-viewer-image"
        />
      );
    case "image_too_large":
      return <FileViewerImageTooLargeState size={state.content.size} />;
    case "sensitive":
      return (
        <FileViewerSensitiveState
          size={state.content.size}
          revealing={props.revealing}
          onReveal={props.onReveal}
        />
      );
    case "binary":
      return <FileViewerBinaryState size={state.content.size} />;
    case "text":
      return null;
  }
}

/** The explorer's right pane: every viewer state for the current target. */
export function FileViewerPane({
  root,
  target,
  onRevealInTree,
  onRefreshTree,
}: FileViewerPaneProps) {
  if (!target) return <FileViewerSelectState />;
  if (target.kind === "outside") {
    return (
      <FileViewerOutsideState
        path={target.path}
        openIn={<OpenInButton root={root} path={target.path} />}
      />
    );
  }
  return (
    <OpenFileView
      key={`${target.path}#${target.nonce}`}
      root={root}
      path={target.path}
      line={target.line}
      findSeed={target.find}
      onRevealInTree={onRevealInTree}
      onRefreshTree={onRefreshTree}
    />
  );
}
