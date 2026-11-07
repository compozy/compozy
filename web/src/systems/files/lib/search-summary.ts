import type { FileSearchResponse } from "../adapters/files-api";
import { splitGlobs } from "./search-globs";

/** The search options that narrow results, as the no-results hint names them. */
export interface SearchFilterOptions {
  regex: boolean;
  caseSensitive: boolean;
  wholeWord: boolean;
  include: string;
  exclude: string;
}

const COUNT = new Intl.NumberFormat("en-US");

function plural(count: number, one: string, many: string): string {
  return `${COUNT.format(count)} ${count === 1 ? one : many}`;
}

/** "6 results in 3 files". */
export function searchSummary(response: FileSearchResponse): string {
  return `${plural(response.totals.matches, "result", "results")} in ${plural(response.totals.files, "file", "files")}`;
}

/** The notes under the summary: caps, timeout, sensitive exclusions, and the walk engine. */
export function searchNotes(response: FileSearchResponse): string[] {
  const notes: string[] = [];
  if (response.truncated) {
    notes.push(`Showing the first ${COUNT.format(2000)} results · refine your search`);
  }
  if (response.timed_out) notes.push("Search stopped after 10 s · results may be incomplete");
  if (response.sensitive_excluded > 0) {
    notes.push(
      `${plural(response.sensitive_excluded, "sensitive file", "sensitive files")} not searched`
    );
  }
  if (response.engine === "walk") notes.push("Not a git repository · searched files on disk");
  return notes;
}

/** Names the active filters for the no-results hint (US-014.EC-1). */
export function activeFilters(options: SearchFilterOptions): string {
  const parts: string[] = [];
  if (options.caseSensitive) parts.push("Match case");
  if (options.wholeWord) parts.push("Whole word");
  if (options.regex) parts.push("Regular expression");
  const include = splitGlobs(options.include);
  const exclude = splitGlobs(options.exclude);
  if (include.length > 0) parts.push(`including ${include.join(", ")}`);
  if (exclude.length > 0) parts.push(`excluding ${exclude.join(", ")}`);
  return parts.join(" · ");
}
