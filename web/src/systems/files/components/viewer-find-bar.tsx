import type { KeyboardEvent } from "react";
import { useEffect, useRef } from "react";

import {
  Button,
  InputGroup,
  InputGroupInput,
  Toggle,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@compozy/ui";
import { CaseSensitive, ChevronDown, ChevronUp, X } from "@compozy/ui/icons";

import type { UseViewerFindResult } from "../hooks/use-viewer-find";
import { findCountLabel } from "../lib/find-in-text";

function BarAction({
  label,
  onClick,
  disabled,
  children,
}: {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  children: React.ReactNode;
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

export interface ViewerFindBarProps {
  find: UseViewerFindResult;
  /** The viewer holds only the first 1 MB of the file (US-015.EC-1). */
  truncated: boolean;
  onClose: () => void;
}

/** Find in file (`_uiux.md` S13), pinned to the viewer's top-right over the code. */
export function ViewerFindBar({ find, truncated, onClose }: ViewerFindBarProps) {
  const inputRef = useRef<HTMLInputElement | null>(null);
  useEffect(() => {
    inputRef.current?.focus();
    inputRef.current?.select();
  }, [find.focusToken]);

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === "Enter" || event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      if (event.key === "ArrowUp" || (event.key === "Enter" && event.shiftKey)) find.previous();
      else find.next();
    } else if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      onClose();
    }
  };
  const label = findCountLabel(find);
  const missing = label === "No results";

  return (
    <div className="relative h-0">
      <search
        aria-label="Find in file"
        data-testid="viewer-find-bar"
        className="absolute top-2 right-3 z-10 flex w-[min(24rem,calc(100%-1.5rem))] flex-col gap-1 rounded-md border border-line bg-popover p-1 shadow-pop"
      >
        <div className="flex items-center gap-0.5">
          <InputGroup className="h-control-compact min-w-0 flex-1">
            <InputGroupInput
              ref={inputRef}
              aria-label="Find"
              placeholder="Find"
              value={find.query}
              aria-invalid={missing || undefined}
              onChange={event => find.setQuery(event.target.value)}
              onKeyDown={onKeyDown}
              data-testid="viewer-find-input"
            />
          </InputGroup>
          <span
            className="shrink-0 px-1 text-xs whitespace-nowrap text-muted-foreground tabular-nums"
            aria-live="polite"
            data-testid="viewer-find-count"
          >
            {label}
          </span>
          <Tooltip>
            <TooltipTrigger
              render={
                <Toggle
                  size="sm"
                  aria-label="Match case"
                  pressed={find.matchCase}
                  onPressedChange={find.setMatchCase}
                  className="h-6 min-w-6 rounded-xxs px-1"
                />
              }
            >
              <CaseSensitive aria-hidden className="size-3.5" />
            </TooltipTrigger>
            <TooltipContent>Match case</TooltipContent>
          </Tooltip>
          <BarAction
            label="Previous match"
            onClick={find.previous}
            disabled={find.matches.length === 0}
          >
            <ChevronUp aria-hidden className="size-3.5" />
          </BarAction>
          <BarAction label="Next match" onClick={find.next} disabled={find.matches.length === 0}>
            <ChevronDown aria-hidden className="size-3.5" />
          </BarAction>
          <BarAction label="Close find" onClick={onClose}>
            <X aria-hidden className="size-3.5" />
          </BarAction>
        </div>
        {truncated ? (
          <p className="px-1.5 text-xs text-muted-foreground" data-testid="viewer-find-truncated">
            Searching the first 1 MB
          </p>
        ) : null}
      </search>
    </div>
  );
}
