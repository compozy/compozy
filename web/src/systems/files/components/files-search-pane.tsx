import type { KeyboardEvent } from "react";
import { useEffect, useRef } from "react";

import {
  Button,
  Input,
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
  Toggle,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { CaseSensitive, Ellipsis, Regex, WholeWord } from "@compozy/ui/icons";

import { cn } from "@/lib/utils";

import type { FileSearchFile, FileSearchMatch, FilesRootRef } from "../adapters/files-api";
import { useFilesSearch, type FilesSearchState } from "../hooks/use-files-search";
import { useFilesSearchPrefs, type FilesSearchOptions } from "../hooks/use-files-search-prefs";
import { useSearchHistoryCursor } from "../hooks/use-search-history-cursor";
import type { ViewerFindSeed } from "../lib/file-viewer-target";
import { firstMatchedText } from "../lib/search-excerpt";
import { FilesSearchResults } from "./files-search-results";
import { FilesSearchStatus } from "./files-search-status";

export interface FilesSearchPaneProps {
  root: FilesRootRef;
  /** Bumps when Cmd/Ctrl+Shift+F asks for the query field. */
  focusToken: number;
  onOpenMatch: (path: string, line: number, find: ViewerFindSeed) => void;
}

function OptionToggle({
  label,
  pressed,
  onPressedChange,
  children,
}: {
  label: string;
  pressed: boolean;
  onPressedChange: (next: boolean) => void;
  children: React.ReactNode;
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Toggle
            size="sm"
            aria-label={label}
            pressed={pressed}
            onPressedChange={onPressedChange}
            className="h-6 min-w-6 rounded-xxs px-1"
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}

function findSeed(options: FilesSearchOptions, pattern: string, match: FileSearchMatch) {
  // Find in file is literal; a regex search seeds it with the text the regex matched.
  if (!options.regex) return { query: pattern, matchCase: options.caseSensitive };
  return { query: firstMatchedText(match) || pattern, matchCase: true };
}

/**
 * The explorer's Search pane (`_uiux.md` S12): query with Match case / Whole word / Regex,
 * optional include and exclude globs, a summary, and grouped results. Options and the last
 * ten queries persist per root.
 */
export function FilesSearchPane({ root, focusToken, onOpenMatch }: FilesSearchPaneProps) {
  const prefs = useFilesSearchPrefs(root);
  const { options, setOptions } = prefs;
  const search = useFilesSearch(root, options);
  const inputRef = useRef<HTMLInputElement | null>(null);
  const history = useSearchHistoryCursor(prefs.history, search.setDraft);

  useEffect(() => {
    if (focusToken === 0) return;
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [focusToken]);

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter") {
      event.preventDefault();
      search.submit();
      if (search.draft.trim()) prefs.recordQuery(search.draft);
    } else if (event.key === "ArrowUp" || event.key === "ArrowDown") {
      event.preventDefault();
      history.step(event.key === "ArrowUp" ? 1 : -1);
    }
  };
  const openMatch = (file: FileSearchFile, match: FileSearchMatch) => {
    const pattern = search.submitted?.pattern ?? search.draft;
    prefs.recordQuery(pattern);
    onOpenMatch(file.path, match.line, findSeed(options, pattern, match));
  };
  const invalid = search.state.kind === "invalid";

  return (
    <div className="flex min-h-0 flex-1 flex-col" data-testid="files-search">
      <div className="flex shrink-0 flex-col gap-1.5 border-b border-line-soft px-2 pb-2">
        <div className="flex items-center gap-1">
          <InputGroup className="h-control-compact">
            <InputGroupInput
              ref={inputRef}
              aria-label="Search"
              placeholder="Search"
              value={search.draft}
              aria-invalid={invalid || undefined}
              aria-describedby={invalid ? "files-search-error" : undefined}
              onChange={event => {
                history.reset();
                search.setDraft(event.target.value);
              }}
              onKeyDown={onKeyDown}
              data-testid="files-search-input"
            />
            <InputGroupAddon align="inline-end" className="gap-0.5">
              <OptionToggle
                label="Match case"
                pressed={options.caseSensitive}
                onPressedChange={caseSensitive => setOptions({ caseSensitive })}
              >
                <CaseSensitive aria-hidden className="size-3.5" />
              </OptionToggle>
              <OptionToggle
                label="Match whole word"
                pressed={options.wholeWord}
                onPressedChange={wholeWord => setOptions({ wholeWord })}
              >
                <WholeWord aria-hidden className="size-3.5" />
              </OptionToggle>
              <OptionToggle
                label="Use regular expression"
                pressed={options.regex}
                onPressedChange={regex => setOptions({ regex })}
              >
                <Regex aria-hidden className="size-3.5" />
              </OptionToggle>
            </InputGroupAddon>
          </InputGroup>
          <Button
            type="button"
            variant="quiet"
            size="icon-xs"
            aria-label="Toggle search details"
            aria-expanded={options.showGlobs}
            onClick={() => setOptions({ showGlobs: !options.showGlobs })}
            data-testid="files-search-details"
          >
            <Ellipsis aria-hidden className="size-3.5" />
          </Button>
        </div>
        {invalid ? (
          <p
            id="files-search-error"
            role="alert"
            className="text-xs break-words text-destructive"
            data-testid="files-search-error"
          >
            {(search.state as Extract<FilesSearchState, { kind: "invalid" }>).message}
          </p>
        ) : null}
        {options.showGlobs ? (
          <div className="flex flex-col gap-1.5">
            <Input
              aria-label="files to include"
              placeholder="files to include"
              value={options.include}
              onChange={event => setOptions({ include: event.target.value })}
              className={cn("h-control-compact font-mono text-xs")}
              data-testid="files-search-include"
            />
            <Input
              aria-label="files to exclude"
              placeholder="files to exclude"
              value={options.exclude}
              onChange={event => setOptions({ exclude: event.target.value })}
              className="h-control-compact font-mono text-xs"
              data-testid="files-search-exclude"
            />
          </div>
        ) : null}
      </div>
      <FilesSearchStatus state={search.state} draft={search.draft} options={options} />
      {search.state.kind === "results" && search.state.response.files.length > 0 ? (
        <FilesSearchResults
          key={JSON.stringify(search.submitted)}
          files={search.state.response.files}
          dimmed={search.state.searching}
          onOpenMatch={openMatch}
        />
      ) : null}
    </div>
  );
}
