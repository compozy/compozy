/**
 * The Terminal app route a bell row opens. The pin lives on that terminal;
 * answering never happens in the popover.
 */
export function terminalAttentionLocation(terminalId: string): {
  pathname: string;
  search: Record<string, never>;
} {
  return {
    pathname: `/terminal/${encodeURIComponent(terminalId)}`,
    search: {},
  };
}
