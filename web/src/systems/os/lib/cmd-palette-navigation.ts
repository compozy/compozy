/** Resolve the same route for local palette actions and daemon client commands. */
export function paletteNavigationTarget(args: Readonly<Record<string, unknown>>) {
  const pathname = args.pathname;
  const search: Record<string, string> = {};
  for (const [key, value] of Object.entries(args)) {
    if (key === "pathname") continue;
    if (typeof value === "string" && value.trim() !== "") search[key] = value;
    else if (typeof value === "number" || typeof value === "boolean") search[key] = String(value);
  }
  return {
    pathname: typeof pathname === "string" && pathname.trim() !== "" ? pathname : null,
    search,
  };
}
