"use client";

import type { ReactNode } from "react";
import {
  SearchDialog,
  SearchDialogClose,
  SearchDialogContent,
  SearchDialogFooter,
  SearchDialogHeader,
  SearchDialogIcon,
  SearchDialogInput,
  SearchDialogList,
  SearchDialogOverlay,
  TagsList,
  TagsListItem,
} from "fumadocs-ui/components/dialog/search";
import type { SearchLink, SharedProps, TagItem } from "fumadocs-ui/contexts/search";
import {
  useSiteSearchDialogState,
  type SiteSearchType,
} from "@/components/site/hooks/use-site-search-dialog-state";

export { SiteSearchProvider } from "@/components/site/site-search-provider";

export interface SiteSearchDialogProps extends SharedProps {
  allowClear?: boolean;
  api?: string;
  defaultTag?: string;
  delayMs?: number;
  footer?: ReactNode;
  links?: SearchLink[];
  tags?: TagItem[];
  type?: SiteSearchType;
}

export function SiteSearchDialog({
  defaultTag,
  tags = [],
  api,
  delayMs,
  type = "fetch",
  allowClear = false,
  links = [],
  footer,
  ...props
}: SiteSearchDialogProps) {
  const { defaultItems, handleSearchChange, isLoading, results, search, setTag, tag } =
    useSiteSearchDialogState({
      api,
      defaultTag,
      delayMs,
      links,
      type,
    });

  return (
    <SearchDialog
      search={search}
      onSearchChange={handleSearchChange}
      isLoading={isLoading}
      {...props}
    >
      <SearchDialogOverlay />
      <SearchDialogContent>
        <SearchDialogHeader>
          <SearchDialogIcon />
          <SearchDialogInput />
          <SearchDialogClose />
        </SearchDialogHeader>
        <SearchDialogList items={results !== "empty" ? results : defaultItems} />
      </SearchDialogContent>
      <SearchDialogFooter>
        {tags.length > 0 && (
          <TagsList tag={tag} onTagChange={setTag} allowClear={allowClear}>
            {tags.map(tag => (
              <TagsListItem key={tag.value} value={tag.value}>
                {tag.name}
              </TagsListItem>
            ))}
          </TagsList>
        )}
        {footer}
      </SearchDialogFooter>
    </SearchDialog>
  );
}
