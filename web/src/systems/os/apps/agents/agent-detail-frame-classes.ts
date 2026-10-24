import { cn, PAGE_CONTENT_GUTTER } from "@compozy/ui";

// The agent page's frame, shared by the location (loaded and loading) and by the window's loading
// fallback so the three hold the same geometry. In a module of their own so the component file
// stays hot-reloadable.

export const AGENT_DETAIL_ROOT_CLASS = "flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden";
export const AGENT_DETAIL_PAGE_CLASS = cn(PAGE_CONTENT_GUTTER, "flex min-h-0 flex-1 flex-col");
/** The tab list and body column: it fills the page and the body scrolls. */
export const AGENT_DETAIL_TABS_CLASS = "min-h-0 flex-1 gap-0";
/** The inline 4px pad keeps card hairlines (shadow rings) clear of the scroll clip. */
export const AGENT_DETAIL_BODY_CLASS =
  "-mx-1 flex min-h-0 flex-1 flex-col overflow-y-auto px-1 py-5";
/** One tab's panel; the tabs primitive adds `flex-1 text-small-body` around it. */
export const AGENT_TAB_PANEL_CLASS = "flex flex-col gap-6";
