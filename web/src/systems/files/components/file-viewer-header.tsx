import type { ReactNode } from "react";

import {
  Button,
  ToggleGroup,
  ToggleGroupItem,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { Copy, FolderTree, RefreshCw } from "@compozy/ui/icons";

import { cn } from "@/lib/utils";

import type { FileViewerMode } from "../lib/file-preview";
import { formatFileSize } from "../lib/format-file-size";

const MODE_LABELS: Record<FileViewerMode, string> = {
  source: "Source",
  preview: "Preview",
  split: "Split",
};

/** Source / Preview / Split, for files with a rendered form (Cmd/Ctrl+Shift+V flips the first two). */
function FileViewerModeToggle({
  mode,
  onModeChange,
}: {
  mode: FileViewerMode;
  onModeChange: (mode: FileViewerMode) => void;
}) {
  return (
    <ToggleGroup
      size="sm"
      aria-label="View"
      value={[mode]}
      onValueChange={value => {
        const next = value[0];
        if (next === "source" || next === "preview" || next === "split") onModeChange(next);
      }}
      data-testid="file-viewer-mode"
    >
      {(Object.keys(MODE_LABELS) as FileViewerMode[]).map(value => (
        <ToggleGroupItem key={value} value={value} data-testid={`file-viewer-mode-${value}`}>
          {MODE_LABELS[value]}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  );
}

function HeaderAction({
  label,
  onClick,
  disabled,
  children,
  testId,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  children: ReactNode;
  testId: string;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="quiet"
            size="icon-xs"
            aria-label={label}
            data-testid={testId}
            disabled={disabled}
            onClick={onClick}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

export interface FileViewerHeaderProps {
  path: string;
  size?: number;
  refreshing?: boolean;
  onCopyPath: () => void;
  onRevealInTree?: () => void;
  onRefresh: () => void;
  /** The Open in slot (desktop only); absent elsewhere. */
  openIn?: ReactNode;
  /** The view mode, for files with a rendered form; the toggle is absent without it. */
  mode?: FileViewerMode;
  onModeChange?: (mode: FileViewerMode) => void;
}

/** Path (directory muted, basename emphasised), size, and the viewer's header actions. */
export function FileViewerHeader({
  path,
  size,
  refreshing = false,
  onCopyPath,
  onRevealInTree,
  onRefresh,
  openIn,
  mode,
  onModeChange,
}: FileViewerHeaderProps) {
  const slash = path.lastIndexOf("/");
  const directory = slash < 0 ? "" : path.slice(0, slash + 1);
  const name = slash < 0 ? path : path.slice(slash + 1);
  return (
    <div
      data-testid="file-viewer-header"
      className="flex h-9 shrink-0 items-center gap-2 border-b border-line-soft px-3"
    >
      <p className="min-w-0 flex-1 truncate font-mono text-xs" title={path}>
        <span className="text-muted-foreground">{directory}</span>
        <span className="text-fg">{name}</span>
      </p>
      {mode && onModeChange ? (
        <FileViewerModeToggle mode={mode} onModeChange={onModeChange} />
      ) : null}
      {size !== undefined ? (
        <span className="shrink-0 text-xs text-muted-foreground tabular-nums">
          {formatFileSize(size)}
        </span>
      ) : null}
      <div className="flex shrink-0 items-center gap-0.5">
        <HeaderAction label="Copy path" testId="file-viewer-copy-path" onClick={onCopyPath}>
          <Copy aria-hidden className="size-3.5" />
        </HeaderAction>
        {onRevealInTree ? (
          <HeaderAction label="Reveal in tree" testId="file-viewer-reveal" onClick={onRevealInTree}>
            <FolderTree aria-hidden className="size-3.5" />
          </HeaderAction>
        ) : null}
        <HeaderAction
          label="Refresh"
          testId="file-viewer-refresh"
          disabled={refreshing}
          onClick={onRefresh}
        >
          <RefreshCw aria-hidden className={cn("size-3.5", refreshing && "animate-spin")} />
        </HeaderAction>
        {openIn}
      </div>
    </div>
  );
}
