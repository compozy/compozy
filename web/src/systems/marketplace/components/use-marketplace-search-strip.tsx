import { useNavigate } from "@tanstack/react-router";

import { ListingToolbar, PillGroup } from "@compozy/ui";

import { useDebouncedInput } from "@/hooks/use-debounced-input";
import { useListingSearchShortcut } from "@/hooks/use-listing-search-shortcut";

import type { MarketplaceKind } from "../lib/marketplace-search";

const KIND_ITEMS = [
  { value: "plugins", label: "Plugins", testId: "marketplace-kind-plugins" },
  { value: "mcps", label: "MCPs", testId: "marketplace-kind-mcps" },
] as const;

interface MarketplaceSearchStripOptions {
  query: string;
  to: "/marketplace" | "/marketplace/installed";
  placeholder: string;
  label: string;
  testId: string;
  /** Browse only: the Plugins | MCPs toggle ahead of the search; `?q=` survives a switch. */
  kind?: MarketplaceKind;
}

/**
 * The strip is the optional kind toggle plus search: `/` focuses, `Esc` clears, and the committed
 * draft lands in `?q=` on the owning route so the API (Browse plugins), the MCP server list
 * (Browse MCPs), or the complete inventory (Installed) does the filtering.
 */
function useMarketplaceSearchStrip({
  query,
  to,
  placeholder,
  label,
  testId,
  kind,
}: MarketplaceSearchStripOptions) {
  const navigate = useNavigate();
  const searchInputRef = useListingSearchShortcut();
  const draft = useDebouncedInput({
    externalValue: query,
    onCommit: value =>
      void navigate({
        replace: true,
        search: { kind: kindParam(kind), q: value.trim() || undefined },
        to,
      }),
  });
  /** Carries the typed draft into the next kind and cancels its pending commit for the old one. */
  const switchKind = (next: MarketplaceKind) => {
    const q = draft.draftValue.trim();
    draft.reset(q);
    void navigate({ search: { kind: kindParam(next), q: q || undefined }, to });
  };
  const toolbar = (
    <ListingToolbar>
      <ListingToolbar.Leading>
        {kind ? (
          <PillGroup<MarketplaceKind>
            aria-label="Show"
            data-testid="marketplace-kind"
            items={KIND_ITEMS}
            onChange={switchKind}
            size="sm"
            value={kind}
          />
        ) : null}
        <ListingToolbar.Search
          aria-label={label}
          containerClassName="w-full max-w-md"
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
  return { clear: draft.clear, navigate, switchKind, toolbar };
}

/** The URL value of a kind: Plugins is the default and is never written. */
function kindParam(kind: MarketplaceKind | undefined) {
  return kind === "mcps" ? ("mcps" as const) : undefined;
}

export { useMarketplaceSearchStrip };
export type { MarketplaceSearchStripOptions };
