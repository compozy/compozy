import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  Pill,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  cn,
} from "@compozy/ui";
import { ChevronDown, ChevronRight, Copy, FileCode, Lock, MoreHorizontal } from "@compozy/ui/icons";

import type { DiffViewerFileHeaderState } from "@compozy/ui";

import { FileMenuItems, canOpenInTargets, type FilesRootRef } from "@/systems/files";

import type { SessionChangeNote } from "../../hooks/use-session-changes";
import { changeStatusLetter, type SessionChangeItem } from "../../lib/session-changes";
import { ChangesIconAction } from "./session-changes-states";

const STATUS_TONE: Record<string, string> = {
  A: "text-success",
  U: "text-muted-foreground",
  D: "text-destructive",
  C: "text-destructive",
};

/** A path as a muted directory and an emphasised basename. */
function SplitPath({ path }: { path: string }) {
  const slash = path.lastIndexOf("/");
  return (
    <>
      <span className="text-muted-foreground">{slash < 0 ? "" : path.slice(0, slash + 1)}</span>
      <span className="font-medium text-fg">{slash < 0 ? path : path.slice(slash + 1)}</span>
    </>
  );
}

type FileNoteReason =
  | "binary"
  | "gated"
  | SessionChangeNote
  | "error"
  | "too_large_to_expand"
  | "sensitive_context";

/**
 * Why a file shows no diff, or why its drawn diff can't expand context; `null` when the patch
 * renders as usual (or is still loading).
 */
function fileNote(
  item: SessionChangeItem,
  state: DiffViewerFileHeaderState,
  note: SessionChangeNote | undefined,
  revealed: boolean
): FileNoteReason | null {
  if (item.file.binary) return "binary";
  if (item.file.sensitive && !revealed) return "gated";
  if (state.status === "error") return "error";
  if (state.status === "unavailable") return note ?? null;
  if (state.status === "ready" && item.expandBlock) {
    return item.expandBlock === "too_large" ? "too_large_to_expand" : "sensitive_context";
  }
  return null;
}

export interface SessionChangeFileHeaderProps {
  item: SessionChangeItem;
  /** The session's files root: the file menu opens paths under it. */
  filesRoot: FilesRootRef;
  /** The first changed line, where Open in Files and the editors land. */
  firstLine: number | undefined;
  state: DiffViewerFileHeaderState;
  note: SessionChangeNote | undefined;
  revealed: boolean;
  selected: boolean;
  onToggleCollapsed: () => void;
  onOpenInViewer: () => void;
  onCopyPath: () => void;
  onReveal: () => void;
  onRetry: () => void;
}

/**
 * One diff's header (US-007.AC-4): status, path (directory muted, basename emphasised, a
 * rename as `old → new`), `+N −M`, collapse, Open in Files, Copy path, the shared file menu
 * in the desktop app — and the reason a file shows no diff: binary, sensitive, too large, or a
 * failed load.
 */
export function SessionChangeFileHeader({
  item,
  filesRoot,
  firstLine,
  state,
  note,
  revealed,
  selected,
  onToggleCollapsed,
  onOpenInViewer,
  onCopyPath,
  onReveal,
  onRetry,
}: SessionChangeFileHeaderProps) {
  const letter = changeStatusLetter(item.file.status);
  const reason = fileNote(item, state, note, revealed);
  return (
    <div
      className={cn(
        "flex flex-col border-b border-line-soft bg-surface-1",
        selected && "bg-surface-2"
      )}
      data-testid="changes-file-header"
      data-path={item.path}
      data-selected={selected ? "true" : undefined}
    >
      <div className="flex h-8 min-w-0 items-center gap-1.5 px-2">
        <Button
          type="button"
          variant="quiet"
          size="icon-xs"
          aria-label={state.collapsed ? `Expand ${item.path}` : `Collapse ${item.path}`}
          aria-expanded={!state.collapsed}
          onClick={onToggleCollapsed}
        >
          {state.collapsed ? (
            <ChevronRight aria-hidden className="size-3.5" />
          ) : (
            <ChevronDown aria-hidden className="size-3.5" />
          )}
        </Button>
        <span
          className={cn(
            "w-3 shrink-0 text-center font-mono text-xs font-semibold",
            STATUS_TONE[letter] ?? "text-warning"
          )}
          title={item.file.status}
        >
          {letter}
        </span>
        {/* A long path loses its start, not its end: siblings that share a long prefix keep
            their nearest folders, file name and extension. `rtl` moves the ellipsis to the
            start; the isolated `ltr` span keeps the path itself left-to-right. */}
        <p
          className="min-w-0 flex-1 truncate text-left font-mono text-xs [direction:rtl]"
          title={item.prevPath ? `${item.prevPath} → ${item.path}` : item.path}
        >
          <span dir="ltr">
            {item.prevPath ? (
              <>
                <SplitPath path={item.prevPath} />
                <span className="text-muted-foreground"> → </span>
              </>
            ) : null}
            <SplitPath path={item.path} />
          </span>
        </p>
        {item.file.generated ? (
          <Pill size="xs" tone="neutral">
            Generated
          </Pill>
        ) : null}
        {item.file.sensitive ? (
          <Lock aria-label="Sensitive" className="size-3 shrink-0 text-muted-foreground" />
        ) : null}
        <span className="flex shrink-0 items-center gap-1 font-mono text-xs tabular-nums">
          <span className="text-success">+{item.file.additions}</span>
          <span className="text-destructive">−{item.file.deletions}</span>
        </span>
        {state.status === "pending" && item.loadable !== false && !state.collapsed ? (
          <Spinner aria-label="Loading diff" className="size-3" />
        ) : null}
        <ChangesIconAction
          label="Open in Files"
          testId="changes-open-in-viewer"
          onClick={onOpenInViewer}
        >
          <FileCode aria-hidden className="size-3.5" />
        </ChangesIconAction>
        <ChangesIconAction label="Copy path" testId="changes-copy-path" onClick={onCopyPath}>
          <Copy aria-hidden className="size-3.5" />
        </ChangesIconAction>
        {/* The file menu adds the editors and the reveal target, which only the desktop app can
            open; a deleted file has nothing on disk to open. */}
        {canOpenInTargets() && item.file.status !== "deleted" ? (
          <ChangeFileMenu
            root={filesRoot}
            path={item.path}
            line={firstLine}
            onOpenInViewer={onOpenInViewer}
          />
        ) : null}
      </div>
      {reason && !state.collapsed ? (
        <FileNote
          reason={reason}
          onReveal={onReveal}
          onRetry={onRetry}
          onOpenInViewer={onOpenInViewer}
        />
      ) : null}
    </div>
  );
}

/** The shared file menu (Files tree row, viewer, folder header), led by Open in Files. */
function ChangeFileMenu({
  root,
  path,
  line,
  onOpenInViewer,
}: {
  root: FilesRootRef;
  path: string;
  line: number | undefined;
  onOpenInViewer: () => void;
}) {
  return (
    <DropdownMenu>
      <Tooltip>
        <TooltipTrigger
          render={
            <DropdownMenuTrigger
              render={
                <Button
                  type="button"
                  variant="quiet"
                  size="icon-xs"
                  aria-label={`More actions for ${path}`}
                  data-testid="changes-file-menu"
                />
              }
            />
          }
        >
          <MoreHorizontal aria-hidden className="size-3.5" />
        </TooltipTrigger>
        <TooltipContent>More actions</TooltipContent>
      </Tooltip>
      <DropdownMenuContent align="end" className="w-auto">
        <FileMenuItems
          host="dropdown"
          root={root}
          request={{ path, line }}
          copyPath={path}
          leading={[
            {
              kind: "item",
              id: "open-in-files",
              label: "Open in Files",
              icon: FileCode,
              onSelect: onOpenInViewer,
            },
          ]}
        />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}

function noteAction(label: string, onClick: () => void) {
  return (
    <Button type="button" variant="outline" size="xs" onClick={onClick}>
      {label}
    </Button>
  );
}

function FileNote({
  reason,
  onReveal,
  onRetry,
  onOpenInViewer,
}: {
  reason: FileNoteReason;
  onReveal: () => void;
  onRetry: () => void;
  onOpenInViewer: () => void;
}) {
  const body = {
    binary: { text: "Binary file changed", button: null },
    gated: {
      text: "Sensitive file · contents hidden",
      button: noteAction("Show contents", onReveal),
    },
    sensitive: {
      text: "Sensitive file · contents hidden",
      button: noteAction("Show contents", onReveal),
    },
    too_large: {
      text: "Diff too large to display",
      button: noteAction("Open in Files", onOpenInViewer),
    },
    no_content: { text: "No content changes", button: null },
    too_large_to_expand: { text: "File too large to expand", button: null },
    sensitive_context: { text: "Sensitive file · context stays hidden", button: null },
    error: { text: "Couldn't load this diff", button: noteAction("Retry", onRetry) },
  }[reason];
  return (
    <div
      className="flex items-center gap-2 px-3 pb-2 pl-10 text-xs text-muted-foreground"
      data-testid="changes-file-note"
      data-reason={reason}
    >
      <span>{body.text}</span>
      {body.button}
    </div>
  );
}
