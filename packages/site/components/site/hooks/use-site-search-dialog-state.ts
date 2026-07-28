"use client";

import { useDocsSearch } from "fumadocs-core/search/client";
import { fetchClient } from "fumadocs-core/search/client/fetch";
import { oramaStaticClient } from "fumadocs-core/search/client/orama-static";
import type { SearchLink } from "fumadocs-ui/contexts/search";
import { useI18n } from "fumadocs-ui/contexts/i18n";
import { useEffect, useRef, useState } from "react";
import { useSiteSearch } from "@/components/site/hooks/use-site-search";

export type SiteSearchType = "fetch" | "static";

type SiteSearchDialogStateOptions = {
  api?: string;
  defaultTag?: string;
  delayMs?: number;
  links?: SearchLink[];
  type?: SiteSearchType;
};

export function useSiteSearchDialogState({
  api,
  defaultTag,
  delayMs,
  links = [],
  type = "fetch",
}: SiteSearchDialogStateOptions) {
  const { locale } = useI18n();
  const { seed, setQuery } = useSiteSearch();
  const [tagState, setTagState] = useState({ defaultTag, value: defaultTag });
  const tag = tagState.defaultTag === defaultTag ? tagState.value : defaultTag;
  const setTag = (value: string | undefined) => setTagState({ defaultTag, value });
  const appliedSeedVersion = useRef(-1);
  const client =
    type === "fetch"
      ? fetchClient({ api, locale, tag })
      : oramaStaticClient({ from: api, locale, tag });
  const { search, setSearch, query } = useDocsSearch({ client, delayMs }, [type, api, locale, tag]);

  useEffect(() => {
    if (appliedSeedVersion.current === seed.version) return;
    appliedSeedVersion.current = seed.version;
    setSearch(seed.query);
    setQuery(seed.query);
  }, [seed.query, seed.version, setQuery, setSearch]);

  const handleSearchChange = (nextQuery: string) => {
    setSearch(nextQuery);
    setQuery(nextQuery);
  };

  const defaultItems =
    links.length === 0
      ? null
      : links.map(([name, link]) => ({
          type: "page" as const,
          id: name,
          content: name,
          url: link,
        }));

  return {
    defaultItems,
    handleSearchChange,
    isLoading: query.isLoading,
    results: query.data,
    search,
    setTag,
    tag,
  };
}
