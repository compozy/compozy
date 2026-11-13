export const LAYOUT_FILE_UNREADABLE = "The selected file could not be read.";
export const LAYOUT_FILE_NOT_A_DOCUMENT = "The selected file is not a layout document.";

/** What the editor says when a document it sent for conversion does not come back. */
export interface LayoutConversionMessages {
  /** The daemon read the document and refused it: a version or shape it does not accept. */
  refused: string;
  /** The check itself did not finish: the daemon was unreachable or answered unexpectedly. */
  failed: string;
}

export const LAYOUT_FILE_CONVERSION: LayoutConversionMessages = {
  refused: "CompozyOS can't open this layout document. It may come from a newer version.",
  failed: "Couldn't check the layout file. Try again.",
};

export const SAVED_LAYOUT_CONVERSION: LayoutConversionMessages = {
  refused: "This saved layout could not be opened.",
  failed: "Couldn't open this saved layout. Try again.",
};

/** A failure the editor already words for people; its message is shown as is. */
export class LayoutImportError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "LayoutImportError";
  }
}

/**
 * Reads an imported file into the wire object the daemon converts. A file that
 * cannot be read, is empty, is not JSON, or is JSON but not an object fails with
 * product wording; parser text never reaches the editor.
 */
export async function readLayoutFile(file: File): Promise<Record<string, unknown>> {
  let text: string;
  try {
    text = await file.text();
  } catch {
    throw new LayoutImportError(LAYOUT_FILE_UNREADABLE);
  }
  let value: unknown;
  try {
    value = JSON.parse(text);
  } catch {
    throw new LayoutImportError(LAYOUT_FILE_NOT_A_DOCUMENT);
  }
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    throw new LayoutImportError(LAYOUT_FILE_NOT_A_DOCUMENT);
  }
  return value as Record<string, unknown>;
}

/**
 * The HTTP status of a daemon answer, read off the error's `status` field: the
 * Settings adapter's `WindowManagerLayoutsApiError` carries it. The field is read
 * structurally, not by class identity, so the rule holds wherever the error was
 * built; transport and schema failures have none.
 */
function daemonStatus(cause: unknown): number | null {
  if (!(cause instanceof Error)) return null;
  const status: unknown = Reflect.get(cause, "status");
  return typeof status === "number" ? status : null;
}

/**
 * Words a conversion failure. The daemon's refusal body carries an error code
 * (`invalid_topology`, `invalid_command`), and transport or schema failures carry
 * engine text, so neither is shown: a 4xx answer reads as `refused`, anything
 * else as `failed`.
 */
export function layoutConversionError(
  cause: unknown,
  messages: LayoutConversionMessages
): LayoutImportError {
  if (cause instanceof LayoutImportError) return cause;
  const status = daemonStatus(cause);
  const refused = status !== null && status >= 400 && status < 500;
  return new LayoutImportError(refused ? messages.refused : messages.failed);
}
