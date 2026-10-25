/**
 * The panel's own geometry, shared by the live panel and the Usage window's loading shape so the
 * two cannot drift: the stack the header, rows and footer sit in, the header with its heading
 * column, and the footer row under its rule. Kept free of imports so a caller that only needs the
 * shapes does not pull the panel's hooks and rows in with them.
 */
export const PROVIDER_USAGE_PANEL_FRAME = "flex min-h-0 min-w-0 flex-col gap-2";

export const PROVIDER_USAGE_HEADER_FRAME = "flex items-start gap-2 px-2 pt-1";

export const PROVIDER_USAGE_HEADING_FRAME = "min-w-0 flex-1 justify-center self-stretch";

export const PROVIDER_USAGE_FOOTER_FRAME =
  "flex items-center justify-between gap-2 border-t border-line-soft px-1 pt-2";
