import type { UIMessage } from "../types";

/*
 * The tool fields a find match names (`field` on a search match) as text: the
 * daemon searched the raw input / output / error, so the reveal shows exactly
 * that payload. Pure over the tool message; the renderers own the display.
 */

/** The daemon's searchable tool fields that live in the collapsed body. */
export type SessionToolBodyField = "input" | "output" | "error";

export function isToolBodyField(field: string | undefined): field is SessionToolBodyField {
  return field === "input" || field === "output" || field === "error";
}

/** The raw input as the daemon searched it: bare JSON, every key. */
export function formatToolInputText(input: Record<string, unknown>): string {
  try {
    return JSON.stringify(input, null, 2);
  } catch {
    return String(input);
  }
}

/** The result's text as the daemon searched it: error, stdout, content, stderr, or the JSON. */
export function formatToolResultText(result: NonNullable<UIMessage["toolResult"]>): string {
  if (result.error) return result.error;
  if (result.stdout) return result.stdout;
  if (result.content) return result.content;
  if (result.stderr) return result.stderr;
  if (result.filePath) return result.filePath;
  try {
    return JSON.stringify(result, null, 2);
  } catch {
    return String(result);
  }
}

/** The field's whole text as the daemon searched it; `null` when the call carries none. */
export function matchedToolFieldText(
  message: UIMessage,
  field: SessionToolBodyField
): string | null {
  switch (field) {
    case "input":
      return message.toolInput && Object.keys(message.toolInput).length > 0
        ? formatToolInputText(message.toolInput)
        : null;
    case "output":
      return message.toolResult ? formatToolResultText(message.toolResult) : null;
    case "error": {
      const result = message.toolResult;
      if (typeof result?.error === "string" && result.error.length > 0) return result.error;
      if (typeof result?.stderr === "string" && result.stderr.length > 0) return result.stderr;
      return null;
    }
  }
}
