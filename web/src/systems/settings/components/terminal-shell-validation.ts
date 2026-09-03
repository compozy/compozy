export function validateTerminalShell(value: string): string | null {
  if (value === "") return null;
  const trimmed = value.trim();
  const containsSeparator = value.includes("/") || value.includes("\\");
  const absolute =
    value.startsWith("/") || /^[A-Za-z]:[\\/]/.test(value) || value.startsWith("\\\\");
  if (trimmed !== value || value.includes("\0") || (containsSeparator && !absolute)) {
    return "Enter a command name or an absolute path.";
  }
  return null;
}
