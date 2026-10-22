/** The tallest an inline screen grows before it scrolls, in terminal rows. */
export const SESSION_TERMINAL_MAX_ROWS = 8;

const ANSI_SEQUENCE = new RegExp(`${String.fromCharCode(27)}\\[[0-9;?]*[ -/]*[@-~]`, "gu");

/**
 * Rows the recorded screen needs at `columns` wide, capped at the inline max.
 *
 * A finished command's screen is sized to what it printed, so three lines of
 * output take three rows — never an emulator's default 24 with the rest empty.
 */
export function sessionTerminalRows(preview: string, columns: number): number {
  const lines = preview.replace(/(\r?\n)+$/u, "").split(/\r?\n/u);
  let rows = 0;
  for (const line of lines) {
    const visible = line.replace(ANSI_SEQUENCE, "").replace(/\r/gu, "");
    rows += Math.max(1, Math.ceil(visible.length / Math.max(1, columns)));
    if (rows >= SESSION_TERMINAL_MAX_ROWS) return SESSION_TERMINAL_MAX_ROWS;
  }
  return Math.max(1, rows);
}
