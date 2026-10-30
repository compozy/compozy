import { selectionHref, type Selection } from "#gallery/shell/view";
import { useSearch } from "#gallery/use-search";

/**
 * `selectionHref` bound to the current URL as React state, so rows rebuild their links when kept
 * parameters (`theme`, `w`…) change.
 */
export function useSelectionHref() {
  const search = useSearch();
  return (selection: Selection) => selectionHref(selection, search);
}
