import type { ComponentProps } from "react";

import { useSelectionHref } from "#gallery/shell/use-selection-href";
import { plainClick, select, type Selection } from "#gallery/shell/view";

type SelectionLinkProps = Omit<ComponentProps<"a">, "href" | "target"> & {
  to: Selection;
  /** Runs after a plain-click navigation (close a drawer, clear a search). */
  onNavigate?: () => void;
};

/**
 * A link to a gallery selection with its own URL: plain clicks navigate in place with a history
 * entry, modified clicks keep the browser default (new tab).
 */
export function SelectionLink({ to, onNavigate, onClick, ...props }: SelectionLinkProps) {
  const hrefOf = useSelectionHref();
  return (
    <a
      {...props}
      href={hrefOf(to)}
      onClick={event => {
        onClick?.(event);
        if (!plainClick(event)) return;
        select(to);
        onNavigate?.();
      }}
    />
  );
}
