import type { Image, MetaField, ViewBadge } from "@compozy/extension-sdk";
import { createElement } from "react";
import type { ReactElement, ReactNode } from "react";

export interface DetailProps {
  isLoading?: boolean;
  markdown?: string;
  metadata?: MetaField[];
  actions?: ReactElement;
}

export function Detail({ actions, ...props }: DetailProps): ReactElement {
  return createElement("view-detail", props, actions);
}

export interface GridProps {
  children?: ReactNode;
  isLoading?: boolean;
  searchText?: string;
  searchBarPlaceholder?: string;
  throttle?: boolean | number;
  filtering?: boolean;
  complete?: boolean;
  columns?: number;
  onSearchTextChange?: (value: string, eventCount: number) => void | Promise<void>;
  onSelectionChange?: (itemID: string) => void | Promise<void>;
}

export interface GridSectionProps {
  title?: string;
  children?: ReactNode;
}

export interface GridItemProps {
  id: string;
  title: string;
  image: Image;
  badge?: ViewBadge;
  actions?: ReactElement;
}

function GridRoot({ children, ...props }: GridProps): ReactElement {
  return createElement("view-grid", props, children);
}

function GridSection({ children, ...props }: GridSectionProps): ReactElement {
  return createElement("view-grid-section", props, children);
}

function GridItem({ actions, ...props }: GridItemProps): ReactElement {
  return createElement("view-grid-item", props, actions);
}

export const Grid = Object.assign(GridRoot, { Section: GridSection, Item: GridItem });
