import type { KeyboardEvent } from "react";
import { useEffect, useState } from "react";

import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@compozy/ui";
import { File, Folder, Lock } from "@compozy/ui/icons";

import type { FileFindMatch, FilesRootRef } from "../adapters/files-api";
import { useFilesFind, type FilesFindState } from "../hooks/use-files-find";
import { byteRangesToCharRanges, type CharRange } from "../lib/byte-ranges";
import { splitPath } from "../lib/go-to-file-query";
import { HighlightedText } from "./highlighted-text";

export interface GoToFileBoxProps {
  root: FilesRootRef;
  /** Recently opened files for this root, newest first. */
  recent: readonly string[];
  onClose: () => void;
  onOpenFile: (path: string, line?: number) => void;
  onRevealFolder: (path: string) => void;
}

const COUNT = new Intl.NumberFormat("en-US");

/** Splits full-path ranges into the basename's and the directory's. */
function pathRanges(path: string, ranges: readonly CharRange[]) {
  const { name, directory } = splitPath(path);
  const offset = path.length - name.length;
  const nameRanges: CharRange[] = [];
  for (const [start, end] of ranges) {
    if (end > offset) nameRanges.push([Math.max(0, start - offset), end - offset]);
  }
  return {
    name,
    directory,
    nameRanges,
    directoryRanges: ranges.filter(([start]) => start < directory.length),
  };
}

function PathRow({
  path,
  folder,
  sensitive,
  ranges,
  onSelect,
}: {
  path: string;
  folder: boolean;
  sensitive: boolean;
  ranges: readonly CharRange[];
  onSelect: () => void;
}) {
  const parts = pathRanges(path, ranges);
  const Icon = folder ? Folder : File;
  return (
    <CommandItem value={path} onSelect={onSelect} data-testid="go-to-file-row" data-path={path}>
      <Icon aria-hidden className="size-3.5 text-muted-foreground" />
      <HighlightedText
        text={parts.name}
        ranges={parts.nameRanges}
        className="shrink-0 truncate font-medium text-fg"
      />
      {sensitive ? <Lock aria-label="Sensitive" className="size-3 text-muted-foreground" /> : null}
      <HighlightedText
        text={parts.directory}
        ranges={parts.directoryRanges}
        className="min-w-0 truncate text-xs text-muted-foreground"
      />
    </CommandItem>
  );
}

function Notice({ children, testId }: { children: React.ReactNode; testId: string }) {
  return (
    <p className="px-3 py-6 text-center text-small-body text-muted-foreground" data-testid={testId}>
      {children}
    </p>
  );
}

function footerText(state: FilesFindState, line?: number): string | null {
  const parts: string[] = [];
  if (state.kind === "results") {
    const indexed = `Indexed ${COUNT.format(state.response.indexed_files)} files`;
    parts.push(
      state.response.index_truncated ? `${indexed} · some files aren't searchable by name` : indexed
    );
  }
  if (line !== undefined) parts.push(`Opens at line ${COUNT.format(line)}`);
  return parts.length > 0 ? parts.join(" · ") : null;
}

/**
 * Go to file (`_uiux.md` S11): a quick-open box anchored to the explorer, ranked by the daemon.
 * `name:42` opens at line 42; a folder is revealed and expanded in the tree.
 */
export function GoToFileBox({
  root,
  recent,
  onClose,
  onOpenFile,
  onRevealFolder,
}: GoToFileBoxProps) {
  const [input, setInput] = useState("");
  // Closing returns focus where it was, so explorer shortcuts keep working (no focus trap).
  // Read during the first render, before the input's autoFocus moves focus.
  const [previous] = useState(() =>
    typeof document === "undefined" ? null : document.activeElement
  );
  useEffect(
    () => () => {
      if (previous instanceof HTMLElement && previous.isConnected) previous.focus();
    },
    [previous]
  );
  const { query, state } = useFilesFind(root, input);
  const open = (match: Pick<FileFindMatch, "path" | "kind">) => {
    onClose();
    if (match.kind === "directory") onRevealFolder(match.path);
    else onOpenFile(match.path, query.line);
  };
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    event.preventDefault();
    event.stopPropagation();
    onClose();
  };
  const footer = footerText(state, query.line);

  return (
    <div className="absolute inset-0 z-30" data-testid="go-to-file">
      <div aria-hidden className="absolute inset-0" onMouseDown={onClose} />
      <Command
        label="Go to file"
        shouldFilter={false}
        loop
        onKeyDown={onKeyDown}
        className="absolute inset-x-0 top-2 mx-auto h-auto w-[min(36rem,calc(100%-1rem))] border border-line shadow-pop"
      >
        <CommandInput
          autoFocus
          variant="quiet"
          aria-label="Go to file"
          placeholder="Go to file…"
          value={input}
          onValueChange={setInput}
          data-testid="go-to-file-input"
        />
        <CommandList aria-busy={state.kind === "results" && state.refreshing}>
          {state.kind === "recent" ? (
            recent.length > 0 ? (
              <CommandGroup heading="Recent">
                {recent.map(path => (
                  <PathRow
                    key={path}
                    path={path}
                    folder={false}
                    sensitive={false}
                    ranges={[]}
                    onSelect={() => open({ path, kind: "file" })}
                  />
                ))}
              </CommandGroup>
            ) : (
              <Notice testId="go-to-file-no-recent">No recently opened files</Notice>
            )
          ) : null}
          {state.kind === "loading" ? (
            <Notice testId="go-to-file-loading">Loading files…</Notice>
          ) : null}
          {state.kind === "unavailable" ? (
            <Notice testId="go-to-file-unavailable">This worktree is no longer on disk</Notice>
          ) : null}
          {state.kind === "error" ? (
            <Notice testId="go-to-file-error">{state.message}</Notice>
          ) : null}
          {state.kind === "results" ? (
            <>
              <CommandEmpty data-testid="go-to-file-empty">
                No files match <code className="font-mono">{query.name}</code>
              </CommandEmpty>
              {state.response.results.map(match => (
                <PathRow
                  key={match.path}
                  path={match.path}
                  folder={match.kind === "directory"}
                  sensitive={match.sensitive === true}
                  ranges={byteRangesToCharRanges(match.path, match.match_ranges)}
                  onSelect={() => open(match)}
                />
              ))}
            </>
          ) : null}
        </CommandList>
        {footer ? (
          <p
            className="border-t border-line-soft px-3 py-1.5 text-xs text-muted-foreground"
            data-testid="go-to-file-footer"
          >
            {footer}
          </p>
        ) : null}
      </Command>
    </div>
  );
}
