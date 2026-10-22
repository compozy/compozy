/** The tallest an inline screen grows before it scrolls, in terminal rows. */
export const SESSION_TERMINAL_MAX_ROWS = 8;

const ESC = String.fromCharCode(27);
const ANSI_SEQUENCE = new RegExp(
  `${ESC}\\[[0-?]*[ -/]*[@-~]|${ESC}\\][\\s\\S]*?(?:\\u0007|${ESC}\\\\)`,
  "gu"
);
const ZERO_WIDTH = /[\p{Mark}\p{Cf}\p{Cc}]/u;
const WIDE =
  /[\u1100-\u115f\u2329\u232a\u2e80-\ua4cf\uac00-\ud7a3\uf900-\ufaff\ufe10-\ufe19\ufe30-\ufe6f\uff01-\uff60\uffe0-\uffe6\u{20000}-\u{3fffd}]|\p{Extended_Pictographic}/u;
const graphemes = new Intl.Segmenter(undefined, { granularity: "grapheme" });

/** Rows occupied by recorded terminal cells, capped at the inline maximum. */
export function sessionTerminalRows(preview: string, columns: number): number {
  const width = Math.max(1, columns);
  const lines = preview
    .replace(ANSI_SEQUENCE, "")
    .replace(/(\r?\n)+$/u, "")
    .split(/\r?\n/u);
  let rows = 0;
  for (const line of lines) {
    let cells = 0;
    let extent = 0;
    for (const { segment } of graphemes.segment(line)) {
      if (segment === "\r") cells = 0;
      else if (segment === "\b") cells = Math.max(0, cells - 1);
      else if (segment === "\t") cells += 8 - (cells % 8);
      else {
        const visible = [...segment].find(character => !ZERO_WIDTH.test(character));
        if (visible) cells += WIDE.test(visible) ? 2 : 1;
      }
      extent = Math.max(extent, cells);
    }
    rows += Math.max(1, Math.ceil(extent / width));
    if (rows >= SESSION_TERMINAL_MAX_ROWS) return SESSION_TERMINAL_MAX_ROWS;
  }
  return Math.max(1, rows);
}
