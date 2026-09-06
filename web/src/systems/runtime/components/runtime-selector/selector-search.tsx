import { Plus, RefreshCw, Search, X } from "lucide-react";
import type { ComponentProps, KeyboardEventHandler, RefObject } from "react";

import { cn } from "@compozy/ui";

interface SelectorSearchProps {
  exactEntry: boolean;
  allowCustomProvider: boolean;
  searchRef: RefObject<HTMLInputElement | null>;
  exactInputId: string;
  listId: string;
  activeDescendant: string | undefined;
  query: string;
  onQueryChange: (query: string) => void;
  onKeyDown: KeyboardEventHandler<HTMLInputElement>;
  onCancelExactEntry: () => void;
  onRefreshCatalog?: () => void;
  refreshing: boolean;
}

export function SelectorSearch({
  exactEntry,
  allowCustomProvider,
  searchRef,
  exactInputId,
  listId,
  activeDescendant,
  query,
  onQueryChange,
  onKeyDown,
  onCancelExactEntry,
  onRefreshCatalog,
  refreshing,
}: SelectorSearchProps) {
  const fieldProps: ComponentProps<"input"> = exactEntry
    ? {
        id: exactInputId,
        placeholder: allowCustomProvider ? "provider/model" : "composer-2.5",
      }
    : {
        role: "combobox",
        "aria-label": "Search models and providers",
        "aria-expanded": true,
        "aria-controls": listId,
        "aria-autocomplete": "list",
        "aria-activedescendant": activeDescendant,
        "aria-keyshortcuts": "Alt+F",
        placeholder: "Search models, providers…",
      };

  return (
    <div className="flex h-9 shrink-0 items-center gap-2 border-b border-line-soft px-3">
      {exactEntry ? (
        <>
          <Plus aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
          <label htmlFor={exactInputId} className="shrink-0 text-badge font-medium text-fg">
            {allowCustomProvider ? "Exact runtime ID" : "Exact model ID"}
          </label>
        </>
      ) : (
        <Search aria-hidden="true" className="size-3.5 shrink-0 text-subtle" />
      )}
      <input
        ref={searchRef}
        {...fieldProps}
        type="text"
        value={query}
        onChange={event => onQueryChange(event.target.value)}
        onKeyDown={onKeyDown}
        autoComplete="off"
        spellCheck={false}
        data-testid="runtime-selector-search"
        className="min-w-0 flex-1 bg-transparent text-small-body text-fg-strong outline-none placeholder:text-subtle"
      />
      {exactEntry ? (
        <button
          type="button"
          aria-label="Return to model search"
          onClick={onCancelExactEntry}
          className="grid size-6 shrink-0 place-items-center rounded-sm text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:ring-2 focus-visible:ring-accent"
        >
          <X aria-hidden="true" className="size-3.5" />
        </button>
      ) : onRefreshCatalog ? (
        <button
          type="button"
          aria-label="Refresh model catalog"
          title="Refresh catalog"
          data-testid="runtime-selector-refresh"
          disabled={refreshing}
          onClick={() => onRefreshCatalog()}
          className="grid size-6 shrink-0 place-items-center rounded-sm text-subtle outline-none transition-colors hover:bg-row-hover hover:text-fg-strong focus-visible:bg-row-hover focus-visible:ring-2 focus-visible:ring-accent disabled:cursor-not-allowed"
        >
          <RefreshCw aria-hidden="true" className={cn("size-3.5", refreshing && "animate-spin")} />
        </button>
      ) : null}
    </div>
  );
}
