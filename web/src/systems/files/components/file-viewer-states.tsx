import type { ReactNode } from "react";

import { Button, Empty, SkeletonText } from "@compozy/ui";
import { AlertCircle, Ban, File, FileX, FolderX, ImageIcon, Lock } from "@compozy/ui/icons";

import { formatFileSize } from "../lib/format-file-size";

export function FileViewerSelectState() {
  return (
    <Empty
      size="compact"
      icon={File}
      title="Select a file"
      description="Pick a file in the tree to read it here."
      data-testid="file-viewer-empty"
    />
  );
}

export function FileViewerLoadingState() {
  return (
    <div className="flex flex-col gap-2 p-4" data-testid="file-viewer-loading" aria-busy>
      <SkeletonText lines={8} />
    </div>
  );
}

export function FileViewerOutsideState({ path, openIn }: { path: string; openIn?: ReactNode }) {
  return (
    <Empty
      size="compact"
      icon={Ban}
      title="Outside this project"
      description={<span className="font-mono break-all">{path}</span>}
      hint="Only files inside this project open in the viewer."
      action={openIn}
      data-testid="file-viewer-outside"
    />
  );
}

export function FileViewerNotFoundState({
  path,
  onRefreshTree,
}: {
  path: string;
  onRefreshTree?: () => void;
}) {
  return (
    <Empty
      size="compact"
      icon={FileX}
      title="File not found"
      description={<span className="font-mono break-all">{path}</span>}
      action={
        onRefreshTree ? (
          <Button type="button" variant="outline" size="sm" onClick={onRefreshTree}>
            Refresh tree
          </Button>
        ) : undefined
      }
      data-testid="file-viewer-not-found"
    />
  );
}

export function FileViewerMessageState({
  title,
  description,
  testId,
  onRetry,
}: {
  title: string;
  description?: ReactNode;
  testId: string;
  onRetry?: () => void;
}) {
  return (
    <Empty
      size="compact"
      icon={AlertCircle}
      title={title}
      description={description}
      action={
        onRetry ? (
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            Try again
          </Button>
        ) : undefined
      }
      data-testid={testId}
    />
  );
}

export function FileViewerBinaryState({ size }: { size: number }) {
  return (
    <Empty
      size="compact"
      icon={File}
      title="Binary file"
      description={`${formatFileSize(size)} · No preview`}
      data-testid="file-viewer-binary"
    />
  );
}

export function FileViewerImageTooLargeState({ size }: { size: number }) {
  return (
    <Empty
      size="compact"
      icon={ImageIcon}
      title="Image too large to preview"
      description={formatFileSize(size)}
      data-testid="file-viewer-image-too-large"
    />
  );
}

export function FileViewerSensitiveState({
  size,
  revealing,
  error,
  onReveal,
}: {
  size: number;
  revealing: boolean;
  error?: string;
  onReveal: () => void;
}) {
  return (
    <Empty
      size="compact"
      icon={Lock}
      title="Sensitive file"
      description={`${formatFileSize(size)} · Its contents stay hidden until you show them.`}
      hint={error}
      action={
        <Button type="button" size="sm" disabled={revealing} onClick={onReveal}>
          Show contents
        </Button>
      }
      data-testid="file-viewer-sensitive"
    />
  );
}

export function FileTreeUnavailableState() {
  return (
    <Empty
      size="compact"
      icon={FolderX}
      title="This worktree is no longer on disk"
      description="Its directory was removed, so there are no files to show."
      data-testid="files-root-unavailable"
    />
  );
}
