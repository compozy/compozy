import { toast } from "sonner";

export const TERMINAL_JOURNAL_COPY_FAILED = "Copy failed";

async function writeClipboard(text: string): Promise<void> {
  const clipboard = typeof navigator === "undefined" ? undefined : navigator.clipboard;
  if (clipboard === undefined || typeof clipboard.writeText !== "function") {
    throw new Error("clipboard unavailable");
  }
  await clipboard.writeText(text);
}

/**
 * Puts the recorded command on the clipboard.
 *
 * Failure is reported; success is silent — the command is already on the
 * screen, and a toast would only confirm what the person just asked for.
 */
export async function copyTerminalJournalCommand(
  command: string,
  write: (text: string) => Promise<void> = writeClipboard
): Promise<boolean> {
  try {
    await write(command);
    return true;
  } catch {
    toast.error(TERMINAL_JOURNAL_COPY_FAILED);
    return false;
  }
}

export function terminalJournalOutputSummary(outputBytes: number, truncated: boolean): string {
  if (outputBytes === 0) return "No output recorded";
  const count = `${outputBytes.toLocaleString("en-US")} bytes`;
  return truncated ? `${count} · cut off` : count;
}
