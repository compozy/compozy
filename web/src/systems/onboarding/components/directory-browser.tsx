import { ChevronUp, Folder, FolderPlus, House, Plus, Spline } from "lucide-react";
import type { ComponentProps } from "react";

import { Button, Spinner, cn } from "@agh/ui";

import type { FSEntry } from "../types";

interface DirectoryBrowserProps extends ComponentProps<"div"> {
  currentPath: string;
  parentPath: string | null;
  homePath: string | null;
  entries: FSEntry[];
  isBrowsing: boolean;
  browseError: string | null;
  onNavigate: (path: string) => void;
  onGoParent: () => void;
  onGoHome: () => void;
  /**
   * Reports a chosen directory. Selection is a report, not a commit — callers
   * that persist decide when to write.
   */
  onPick: (path: string) => void;
  /** Marks a row as already chosen, disabling its pick control. */
  isPicked: (path: string) => boolean;
  pickPending?: boolean;
  /** Label for the "pick the folder I am standing in" action. */
  pickLabel?: string;
  /** Builds the accessible name for a row's pick control. */
  pickRowLabel?: (name: string) => string;
  testIdPrefix?: string;
}

/**
 * Filesystem root picker: home/up toolbar with a mono path, "use this folder",
 * hover-reveal row picks, and reading/empty/error states.
 *
 * Presentational by contract — it reports a pick and never persists one, so a
 * dialog can gate registration behind its own primary action
 * (`MODAL-STANDARD.md` § Component grammar).
 */
export function DirectoryBrowser({
  currentPath,
  parentPath,
  homePath,
  entries,
  isBrowsing,
  browseError,
  onNavigate,
  onGoParent,
  onGoHome,
  onPick,
  isPicked,
  pickPending = false,
  pickLabel = "Use this folder",
  pickRowLabel = name => `Use ${name}`,
  testIdPrefix = "directory-browser",
  className,
  ...props
}: DirectoryBrowserProps) {
  return (
    <div
      className={cn(
        "flex min-h-0 flex-1 flex-col overflow-hidden rounded-md bg-canvas-soft ring-1 ring-line ring-inset max-md:h-60 max-md:flex-none",
        className
      )}
      data-testid={testIdPrefix}
      {...props}
    >
      <div className="flex flex-none items-center gap-2 border-b border-line px-3 py-2">
        <Button
          aria-label="Go to home directory"
          disabled={!homePath}
          onClick={onGoHome}
          size="icon-sm"
          variant="ghost"
        >
          <House className="size-3.5" />
        </Button>
        <Button
          aria-label="Go to parent directory"
          disabled={!parentPath}
          onClick={onGoParent}
          size="icon-sm"
          variant="ghost"
        >
          <ChevronUp className="size-3.5" />
        </Button>
        <span className="truncate font-mono text-xs text-subtle" title={currentPath}>
          {currentPath || "~"}
        </span>
        <span className="ml-auto">
          <Button
            data-testid={`${testIdPrefix}-use-current`}
            disabled={!currentPath || isPicked(currentPath) || pickPending}
            onClick={() => onPick(currentPath)}
            size="sm"
            variant="outline"
          >
            {pickPending ? <Spinner /> : <Plus className="size-3.5" />}
            {pickLabel}
          </Button>
        </span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto p-1.5">
        {isBrowsing ? (
          <div
            className="flex items-center gap-2 px-2.5 py-6 text-sm text-muted"
            data-testid={`${testIdPrefix}-loading`}
          >
            <Spinner /> Reading directory…
          </div>
        ) : browseError ? (
          <p
            className="px-2.5 py-6 text-sm text-danger"
            data-testid={`${testIdPrefix}-error`}
            role="alert"
          >
            {browseError}
          </p>
        ) : entries.length === 0 ? (
          <p className="px-2.5 py-6 text-sm text-faint" data-testid={`${testIdPrefix}-empty`}>
            No sub-folders here.
          </p>
        ) : (
          entries.map(entry => (
            <div
              className="group flex h-setup-row items-center gap-2.5 rounded px-2 hover:bg-hover"
              key={entry.path}
            >
              <button
                className="flex min-w-0 flex-1 items-center gap-2.5 text-left"
                data-testid={`${testIdPrefix}-entry`}
                onClick={() => onNavigate(entry.path)}
                type="button"
              >
                {entry.is_dir ? (
                  <Folder className="size-4 flex-none text-warning" />
                ) : (
                  <Spline className="size-4 flex-none text-faint" />
                )}
                <span className="truncate text-sm text-fg">{entry.name}</span>
              </button>
              <Button
                aria-label={pickRowLabel(entry.name)}
                className={cn(
                  "opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 focus-visible:opacity-100",
                  isPicked(entry.path) && "opacity-40"
                )}
                disabled={isPicked(entry.path) || pickPending}
                onClick={() => onPick(entry.path)}
                size="icon-sm"
                variant="ghost"
              >
                <FolderPlus className="size-3.5" />
              </Button>
            </div>
          ))
        )}
      </div>
    </div>
  );
}
