import { useQuery } from "@tanstack/react-query";
import { lazy, Suspense, useState } from "react";

import { FileImagePreview, FileMediaPreview } from "@compozy/ui";
import { Info } from "@compozy/ui/icons";

import type { FileContentResponse, FilesRootRef } from "../adapters/files-api";
import { copyFilePath } from "../lib/copy-file-path";
import { mediaPreviewKind, rawFileUrl, textPreviewKind } from "../lib/file-preview";
import { FILE_TEXT_LIMIT, formatFileSize } from "../lib/format-file-size";
import { fileViewerStale, fileViewerState, type FileViewerState } from "../lib/file-viewer-state";
import type { FileViewerMode } from "../lib/file-preview";
import type { FileViewerTarget, ViewerFindSeed } from "../lib/file-viewer-target";
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
import { FileTextView } from "./file-text-view";
import { OpenInButton } from "./open-in-button";

export interface FileViewerPaneProps {
  root: FilesRootRef;
  target: FileViewerTarget | null;
  /** Selects and scrolls the tree to the open file. */
  onRevealInTree?: (path: string) => void;
  /** Refetches the tree listings (the "File not found" recovery). */
  onRefreshTree?: () => void;
  /** Opens another file of the same root, such as a relative link in a Markdown preview. */
  onOpenFile?: (path: string) => void;
}

const FilePdfView = lazy(() => import("./file-pdf-view"));

function basename(path: string): string {
  return path.slice(path.lastIndexOf("/") + 1) || path;
}

/** An SVG's markup, decoded from the image payload for its Source view; `null` if undecodable. */
function svgSource(content: FileContentResponse): string | null {
  if (!content.image_base64 || content.image_mime !== "image/svg+xml") return null;
  try {
    const binary = atob(content.image_base64);
    const bytes = Uint8Array.from(binary, char => char.charCodeAt(0));
    return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return null;
  }
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

/** The latest refresh failed but the file may still exist: the text shown is the last good read. */
function StaleNotice() {
  return (
    <p
      role="status"
      data-testid="file-viewer-stale"
      className="flex items-center gap-2 border-b border-line-soft px-3 py-1.5 text-xs text-muted-foreground"
    >
      <Info aria-hidden className="size-3.5 shrink-0" />
      Couldn't refresh this file. Showing the last loaded version.
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
  onOpenFile,
}: {
  root: FilesRootRef;
  path: string;
  line?: number;
  findSeed?: ViewerFindSeed;
} & Pick<FileViewerPaneProps, "onRevealInTree" | "onRefreshTree" | "onOpenFile">) {
  const [revealed, setRevealed] = useState(false);
  const gated = useQuery(fileContentOptions(root, path, false));
  const reveal = useQuery({ ...fileContentOptions(root, path, true), enabled: revealed });
  const active = revealed ? reveal : gated;
  const state: FileViewerState = fileViewerState(active);
  const stale = fileViewerStale(active);
  const content = "content" in state ? state.content : undefined;
  const header = (mode?: FileViewerMode, onModeChange?: (mode: FileViewerMode) => void) => (
    <FileViewerHeader
      path={path}
      size={content?.size}
      refreshing={active.isFetching && !active.isPending}
      onCopyPath={() => copyFilePath(path)}
      onRevealInTree={onRevealInTree ? () => onRevealInTree(path) : undefined}
      onRefresh={() => void active.refetch()}
      openIn={<OpenInButton root={root} path={path} line={line} />}
      mode={mode}
      onModeChange={onModeChange}
    />
  );
  const media = mediaPreviewKind(path);
  const previewKind = textPreviewKind(path);
  const svg = state.kind === "image" && previewKind === "svg" ? svgSource(state.content) : null;
  const text = state.kind === "text" && media === null ? state.text : svg;

  if (text !== null && content) {
    const truncated = content.truncated;
    return (
      <FileTextView
        root={root}
        path={path}
        text={text}
        content={content}
        previewKind={truncated ? null : previewKind}
        line={line}
        findSeed={findSeed}
        header={header}
        notices={
          <>
            {stale ? <StaleNotice /> : null}
            {truncated ? <TruncationNotice size={content.size} /> : null}
          </>
        }
        onOpenFile={onOpenFile}
      />
    );
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      {header()}
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

/**
 * Video, audio and PDF stream from the raw-bytes route whatever the daemon classified them as
 * (an all-ASCII PDF reads as text); the binary state stays the fallback when playback fails.
 */
function mediaBody(content: FileContentResponse, props: { root: FilesRootRef; path: string }) {
  const kind = mediaPreviewKind(props.path);
  if (kind === null) return null;
  const url = rawFileUrl(props.root, props.path);
  const fallback = <FileViewerBinaryState size={content.size} />;
  if (kind === "pdf") {
    return (
      <Suspense fallback={<FileViewerLoadingState />}>
        <FilePdfView url={url} title={basename(props.path)} fallback={fallback} />
      </Suspense>
    );
  }
  return (
    <FileMediaPreview
      kind={kind}
      src={url}
      title={basename(props.path)}
      meta={formatFileSize(content.size)}
      fallback={fallback}
      data-testid={`file-viewer-${kind}`}
    />
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
      // Past the inline cap the image streams from the raw-bytes route instead.
      return (
        <FileImagePreview
          src={rawFileUrl(props.root, props.path)}
          alt={basename(props.path)}
          meta={formatFileSize(state.content.size)}
          fallback={<FileViewerImageTooLargeState size={state.content.size} />}
          data-testid="file-viewer-image"
        />
      );
    case "sensitive":
      return (
        <FileViewerSensitiveState
          size={state.content.size}
          revealing={props.revealing}
          onReveal={props.onReveal}
        />
      );
    case "binary":
      return mediaBody(state.content, props) ?? <FileViewerBinaryState size={state.content.size} />;
    case "text":
      return mediaBody(state.content, props);
  }
}

/** The explorer's right pane: every viewer state for the current target. */
export function FileViewerPane({
  root,
  target,
  onRevealInTree,
  onRefreshTree,
  onOpenFile,
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
      onOpenFile={onOpenFile}
    />
  );
}
