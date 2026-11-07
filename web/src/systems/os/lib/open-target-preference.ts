const KEY = "compozy.open-target";
export function lastOpenTarget(): string {
  try {
    return localStorage.getItem(KEY) || "configured";
  } catch {
    return "configured";
  }
}
export function rememberOpenTarget(id: string): void {
  try {
    localStorage.setItem(KEY, id);
  } catch {
    /* Storage may be disabled. */
  }
}
