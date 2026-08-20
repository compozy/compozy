import type { Chip, DetailBody, ViewBadge } from "@compozy/extension-sdk";
import { createElement } from "react";
import type { ReactElement, ReactNode } from "react";

export interface ListProps {
  children?: ReactNode;
  isLoading?: boolean;
  searchText?: string;
  searchBarPlaceholder?: string;
  throttle?: boolean | number;
  filtering?: boolean;
  complete?: boolean;
  chips?: Chip[];
  activeChip?: string | null;
  pagination?: {
    hasMore: boolean;
    pageSize?: number;
    onLoadMore?: () => void | Promise<void>;
  };
  onSearchTextChange?: (value: string, eventCount: number) => void | Promise<void>;
  onChipToggle?: (chip: string | null) => void | Promise<void>;
  onSelectionChange?: (itemID: string) => void | Promise<void>;
}

export interface ListSectionProps {
  title?: string;
  children?: ReactNode;
}

export interface ListItemProps {
  id: string;
  title: string;
  subtitle?: string;
  icon?: string;
  badge?: ViewBadge;
  accessories?: string[];
  keywords?: string[];
  detail?: ReactElement<ListItemDetailProps>;
  actions?: ReactElement;
}

export interface ListItemDetailProps extends Omit<DetailBody, "actions"> {
  actions?: ReactElement;
}

export interface ListEmptyViewProps {
  title: string;
  hint?: string;
  icon?: string;
}

function ListRoot({ children, ...props }: ListProps): ReactElement {
  return createElement("view-list", props, children);
}

function ListSection({ children, ...props }: ListSectionProps): ReactElement {
  return createElement("view-list-section", props, children);
}

function ListItem({ detail, actions, ...props }: ListItemProps): ReactElement {
  return createElement("view-list-item", props, detail, actions);
}

function ListItemDetail({ actions, ...props }: ListItemDetailProps): ReactElement {
  return createElement("view-list-item-detail", props, actions);
}

function ListEmptyView(props: ListEmptyViewProps): ReactElement {
  return createElement("view-list-empty", props);
}

export const List = Object.assign(ListRoot, {
  Section: ListSection,
  Item: Object.assign(ListItem, { Detail: ListItemDetail }),
  EmptyView: ListEmptyView,
});
