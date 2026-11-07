/** `web/**, *.go` → `["web/**", "*.go"]` (US-014.AC-3). */
export function splitGlobs(value: string): string[] {
  return value
    .split(",")
    .map(glob => glob.trim())
    .filter(glob => glob !== "");
}
