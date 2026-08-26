import { useLayoutEffect, useRef, useState, type KeyboardEvent } from "react";

import {
  Command,
  CommandInput,
  CommandItem,
  CommandList,
  resolveCommandSelection,
} from "@compozy/ui";

import { cn } from "@/lib/utils";

import {
  paletteHeadClass,
  paletteInputRailClass,
  paletteItemClass,
  paletteViewListClass,
} from "../lib/palette-view-inset";
import type { PaletteViewContent, PaletteViewDefinition } from "../lib/palette-view-registry";
import type { PaletteBreadcrumb } from "../lib/palette-view-stack";
import { OsPaletteBreadcrumb } from "./os-palette-breadcrumb";
import { OsPaletteFooter } from "./os-palette-footer";
import { OsPaletteVirtualRows } from "./os-palette-virtual-rows";
import { PALETTE_VIEW_VIRTUAL_THRESHOLD } from "./os-palette-virtual-rows-constants";

export interface OsPaletteViewShellProps {
  definition: PaletteViewDefinition;
  breadcrumb: PaletteBreadcrumb;
  content: PaletteViewContent;
  query: string;
  onQueryChange: (query: string) => void;
  /** Leaves this level for the one below it. */
  onPop: () => void;
  loading?: boolean;
}

function sameRowOrder(left: readonly string[], right: readonly string[]): boolean {
  return left.length === right.length && left.every((value, index) => value === right[index]);
}

/**
 * Everything a palette view does not have to build: the query field, the
 * keyboard contract, the selection, the empty state and the footer.
 *
 * Each level mounts its own `Command` root, which is what keeps results from
 * bleeding across levels (Business Rule 32) — the root palette's entries are
 * not filtered out of this list, they were never registered in it. Filtering
 * happens in the view instead of in cmdk, so what the operator sees is exactly
 * what the selection logic below reasons about.
 */
export function OsPaletteViewShell({
  definition,
  breadcrumb,
  content,
  query,
  onQueryChange,
  onPop,
  loading = false,
}: OsPaletteViewShellProps) {
  const frameRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  useLayoutEffect(() => {
    const frame = frameRef.current;
    const input = inputRef.current;
    if (!frame || !input) return;
    const hiddenAncestor = () => frame.closest("[hidden]") !== null;
    let wasHidden = hiddenAncestor();
    const restore = () => {
      const hidden = hiddenAncestor();
      if (wasHidden && !hidden) input.focus();
      if (!wasHidden && hidden && document.activeElement === input) input.blur();
      wasHidden = hidden;
    };
    const observer = new MutationObserver(restore);
    let node: HTMLElement | null = frame;
    while (node) {
      observer.observe(node, { attributes: true, attributeFilter: ["hidden"] });
      node = node.parentElement;
    }
    return () => observer.disconnect();
  }, []);
  const values = content.rows.map(row => row.value);
  const resetToken = `${query}\u0000${content.resetKey}`;
  const [snapshot, setSnapshot] = useState<{ token: string; values: readonly string[] }>({
    token: resetToken,
    values,
  });
  const [selected, setSelected] = useState(() => values[0] ?? "");
  // Narrowing the list is the operator's own action, so it lands on the first
  // result; anything else is the catalog moving underneath them, and the
  // selection has to survive it (US-029.EC-4, US-030.EC-2).
  if (snapshot.token !== resetToken) {
    setSnapshot({ token: resetToken, values });
    setSelected(values[0] ?? "");
  } else if (!sameRowOrder(snapshot.values, values)) {
    setSnapshot({ token: resetToken, values });
    setSelected(resolveCommandSelection(snapshot.values, values, selected));
  }

  const onInputKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Backspace" || query !== "") return;
    event.preventDefault();
    if (content.onEmptyQueryBackspace()) return;
    onPop();
  };
  const onSelectionChange = (value: string) => {
    setSelected(value);
    content.onSelectionChange?.(value);
  };
  const list =
    content.rows.length > PALETTE_VIEW_VIRTUAL_THRESHOLD ? (
      <OsPaletteVirtualRows
        className={paletteViewListClass}
        note={content.note}
        rows={content.rows}
      />
    ) : (
      <CommandList className={paletteViewListClass}>
        {content.rows.length === 0
          ? content.empty
          : content.rows.map(row => (
              <CommandItem
                key={row.value}
                value={row.value}
                forceMount
                data-testid={row.testId}
                disabled={row.disabled}
                onSelect={row.onSelect}
                className={paletteItemClass(row.twoLine)}
              >
                {row.node}
              </CommandItem>
            ))}
        {content.note}
      </CommandList>
    );

  return (
    <div ref={frameRef} className="contents">
      <Command
        aria-busy={loading}
        data-testid="os-command-palette"
        data-palette-view={definition.id}
        data-palette-kind={content.kind ?? "list"}
        {...(loading ? { "data-palette-loading": "true" } : {})}
        shouldFilter={false}
        value={selected}
        onValueChange={onSelectionChange}
        className={paletteInputRailClass}
      >
        <div className={paletteHeadClass}>
          <OsPaletteBreadcrumb breadcrumb={breadcrumb} />
          <CommandInput
            ref={inputRef}
            autoFocus
            value={query}
            onValueChange={onQueryChange}
            onKeyDown={onInputKeyDown}
            placeholder={definition.placeholder}
          />
        </div>
        {content.header}
        {content.body === undefined ? (
          <div className={cn("flex min-h-0", content.aside && "divide-x divide-line")}>
            <div className="min-w-0 flex-1">{list}</div>
            {content.aside}
          </div>
        ) : (
          <div className={cn(paletteViewListClass, "overflow-auto")} data-palette-view-body>
            {content.body}
          </div>
        )}
        <OsPaletteFooter
          enterHint={definition.enterHint}
          backHint={content.backHint}
          hasFilters={content.header !== null}
        />
      </Command>
    </div>
  );
}
