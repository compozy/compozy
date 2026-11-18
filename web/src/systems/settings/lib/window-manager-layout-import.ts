export const LAYOUT_FILE_UNREADABLE = "The selected file could not be read.";
export const LAYOUT_FILE_NOT_A_DOCUMENT = "The selected file is not a layout document.";

/**
 * Refusals that are not about the document: the project or profile is gone, or
 * this caller may not act on it. Retrying the same file cannot help, and the
 * file is not to blame.
 */
export const LAYOUT_SCOPE_UNAVAILABLE =
  "Layouts for this project and profile aren't available here.";

/** What the editor says when a document it sent for conversion does not come back. */
export interface LayoutConversionMessages {
  /** The daemon read the document and refused it: a version or shape it does not accept. */
  refused: string;
  /** The request was over the daemon's body limit before the document was read. */
  tooLarge: string;
  /**
   * The check did not finish (daemon unreachable, unexpected answer) or ran
   * against a revision the editor has since moved past: trying again works.
   */
  failed: string;
}

export const LAYOUT_FILE_CONVERSION: LayoutConversionMessages = {
  refused: "CompozyOS can't open this layout document. It may come from a newer version.",
  tooLarge: "This layout document is too large to open.",
  failed: "Couldn't check the layout file. Try again.",
};

const SAVED_LAYOUT_REFUSED = "This saved layout could not be opened.";

export const SAVED_LAYOUT_CONVERSION: LayoutConversionMessages = {
  refused: SAVED_LAYOUT_REFUSED,
  tooLarge: SAVED_LAYOUT_REFUSED,
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

type ConversionFailure = keyof LayoutConversionMessages | "scope";

interface DaemonAnswer {
  status: number;
  code: string | null;
}

/**
 * The status and code of a daemon answer, read off the error's fields: the
 * Settings adapter's `WindowManagerLayoutsApiError` carries both. They are read
 * structurally, not by class identity, so the rule holds wherever the error was
 * built; transport and schema failures have neither.
 */
function daemonAnswer(cause: unknown): DaemonAnswer | null {
  if (!(cause instanceof Error)) return null;
  const status: unknown = Reflect.get(cause, "status");
  const code: unknown = Reflect.get(cause, "code");
  if (typeof status !== "number") return null;
  return { status, code: typeof code === "string" ? code : null };
}

/** 4xx answers by what they are about; any status not listed is a check that did not finish. */
const FAILURE_BY_STATUS: Readonly<Record<number, ConversionFailure>> = {
  400: "scope",
  401: "scope",
  403: "scope",
  404: "scope",
  409: "scope",
  413: "tooLarge",
  422: "refused",
};

/** Codes that override their status: a 400 about the document, and a 409 cured by a fresh revision. */
const FAILURE_BY_CODE: Readonly<Record<string, ConversionFailure>> = {
  window_binding_invalid: "refused",
  window_manager_revision_conflict: "failed",
};

function conversionFailure(cause: unknown): ConversionFailure {
  const answer = daemonAnswer(cause);
  if (answer === null) return "failed";
  const byCode = answer.code === null ? undefined : FAILURE_BY_CODE[answer.code];
  return byCode ?? FAILURE_BY_STATUS[answer.status] ?? "failed";
}

/**
 * Words a conversion failure. The daemon's refusal body carries an error code,
 * and transport or schema failures carry engine text, so neither is shown. Only
 * a refusal of the document itself names the document; a missing project or
 * profile, or an authorization refusal, says so without blaming the file.
 */
export function layoutConversionError(
  cause: unknown,
  messages: LayoutConversionMessages
): LayoutImportError {
  if (cause instanceof LayoutImportError) return cause;
  const failure = conversionFailure(cause);
  return new LayoutImportError(failure === "scope" ? LAYOUT_SCOPE_UNAVAILABLE : messages[failure]);
}
