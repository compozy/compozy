import { useNavigate } from "@tanstack/react-router";

import { ListingToolbar } from "@compozy/ui";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";

interface MarketplaceSearchStripOptions {
  query: string;
  to: "/marketplace" | "/marketplace/installed";
  placeholder: string;
  label: string;
  testId: string;
}

/**
 * The strip is search only: `/` focuses, `Esc` clears, and the committed draft lands in `?q=`
 * on the owning route so the API (Browse) or the complete inventory (Installed) does the filtering.
 */
function useMarketplaceSearchStrip({
  query,
  to,
  placeholder,
  label,
  testId,
}: MarketplaceSearchStripOptions) {
  const navigate = useNavigate();
  const searchInputRef = useListingSearchShortcut();
  const draft = useDebouncedInput({
    externalValue: query,
    onCommit: value =>
      void navigate({ replace: true, search: { q: value.trim() || undefined }, to }),
  });
  const toolbar = (
    <ListingToolbar>
      <ListingToolbar.Leading>
        <ListingToolbar.Search
          aria-label={label}
          containerClassName="w-full max-w-105"
          data-testid={testId}
          onChange={draft.setDraftValue}
          onKeyDown={event => {
            if (event.key === "Escape" && draft.draftValue !== "") {
              event.preventDefault();
              draft.clear();
            }
          }}
          placeholder={placeholder}
          ref={searchInputRef}
          value={draft.draftValue}
        />
      </ListingToolbar.Leading>
    </ListingToolbar>
  );
  return { clear: draft.clear, navigate, toolbar };
}

export { useMarketplaceSearchStrip };
export type { MarketplaceSearchStripOptions };
