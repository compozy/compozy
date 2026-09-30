/**
 * The one lucide stroke width (polish contract P4). `UIProvider` sets it as the
 * app-wide lucide default, so a bare `<Search />` and `<Icon as={Search} />`
 * draw the same line; per-call `strokeWidth` is for genuine one-offs only.
 */
export const ICON_STROKE_WIDTH = 1.75;
