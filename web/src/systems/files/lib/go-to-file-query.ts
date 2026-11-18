/** A Go to file query: the name to rank, and the line to open at (`main.go:42`, US-013.AC-5). */
export interface GoToFileQuery {
  name: string;
  line?: number;
}

const LINE_SUFFIX = /^(.+?):(\d+)(?::\d+)?$/;

export function parseGoToFileQuery(input: string): GoToFileQuery {
  const trimmed = input.trim();
  const match = LINE_SUFFIX.exec(trimmed);
  if (!match?.[1] || !match[2]) return { name: trimmed };
  const line = Number(match[2]);
  return line > 0 ? { name: match[1], line } : { name: match[1] };
}

/** `web/src/main.tsx` → `["main.tsx", "web/src"]`. */
export function splitPath(path: string): { name: string; directory: string } {
  const slash = path.lastIndexOf("/");
  return slash < 0
    ? { name: path, directory: "" }
    : { name: path.slice(slash + 1), directory: path.slice(0, slash) };
}
