import type { LinkProps } from "@tanstack/react-router";

/**
 * One route level for the Topbar. The deepest label becomes the shell H1;
 * earlier levels become breadcrumb ancestry.
 */
export interface TopbarCrumbContext {
  /** Crumb label for this route level (static or params-derived). */
  label: string;
  /**
   * Router path for non-leaf levels. Leaf crumbs render as the current page
   * (`aria-current="page"`), so they omit it.
   */
  to?: LinkProps["to"];
  /** Route params for parameterized non-leaf levels (e.g. `/loops/$name`). */
  params?: LinkProps["params"];
  /** Search params for non-leaf levels that scope by query (e.g. marketplace kind). */
  search?: LinkProps["search"];
}

/**
 * Topbar metadata declared by every route's `beforeLoad`. The Topbar owns
 * route ancestry and the current page H1; body chrome owns summaries,
 * search, filters, and scope controls.
 */
export interface TopbarRouteContext {
  crumb: TopbarCrumbContext;
  /**
   * Extra trail level for path segments without their own route file
   * (e.g. the marketplace kind between Marketplace and an entry).
   */
  parentCrumb?: TopbarCrumbContext;
}
