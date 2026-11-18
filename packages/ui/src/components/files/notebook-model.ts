/** One renderable cell output, already reduced to the richest type the viewer shows safely. */
export type NotebookOutputBody =
  | { kind: "text"; text: string; stream?: "stdout" | "stderr" }
  | { kind: "image"; mimeType: string; data: string }
  | { kind: "markdown"; text: string }
  | { kind: "html"; html: string }
  | { kind: "error"; name: string; text: string };

/** An output with a key that stays stable while the notebook text is unchanged. */
export type NotebookOutput = NotebookOutputBody & { id: string };

/** `id` is the nbformat 4.5 cell id, else the cell's position. */
export type NotebookCell = { id: string } & (
  | { kind: "markdown"; source: string }
  | { kind: "raw"; source: string }
  | {
      kind: "code";
      source: string;
      executionCount: number | null;
      outputs: NotebookOutput[];
    }
);

export interface Notebook {
  /** Kernel language for code cells (`python` when the notebook does not say). */
  language: string;
  cells: NotebookCell[];
}

type Json = Record<string, unknown>;

// The order a display bundle is reduced in: images first, then prose, then plain text.
const BITMAP_TYPES = ["image/png", "image/jpeg", "image/gif", "image/webp"] as const;
// eslint-disable-next-line no-control-regex -- ANSI escape sequences start with ESC.
const ANSI = /\u001b\[[0-9;?]*[ -/]*[@-~]/g;

function isObject(value: unknown): value is Json {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** nbformat stores multiline strings either whole or as a list of lines. */
function joinSource(value: unknown): string {
  if (typeof value === "string") return value;
  if (Array.isArray(value)) return value.filter(line => typeof line === "string").join("");
  return "";
}

function utf8Base64(text: string): string {
  let binary = "";
  for (const byte of new TextEncoder().encode(text)) binary += String.fromCharCode(byte);
  return btoa(binary);
}

function stripAnsi(text: string): string {
  return text.replace(ANSI, "");
}

function displayOutput(data: Json): NotebookOutputBody | null {
  for (const mimeType of BITMAP_TYPES) {
    const value = joinSource(data[mimeType]).replace(/\s+/g, "");
    if (value) return { kind: "image", mimeType, data: value };
  }
  const svg = joinSource(data["image/svg+xml"]);
  if (svg) return { kind: "image", mimeType: "image/svg+xml", data: utf8Base64(svg) };
  const markdown = joinSource(data["text/markdown"]);
  if (markdown) return { kind: "markdown", text: markdown };
  // Rich HTML (a DataFrame table) wins over its `text/plain` repr, as in Jupyter.
  const html = joinSource(data["text/html"]);
  if (html) return { kind: "html", html };
  const plain = joinSource(data["text/plain"]);
  if (plain) return { kind: "text", text: stripAnsi(plain) };
  if (data["application/json"] !== undefined) {
    return { kind: "text", text: JSON.stringify(data["application/json"], null, 2) };
  }
  return null;
}

function parseOutput(output: unknown): NotebookOutputBody | null {
  if (!isObject(output)) return null;
  switch (output.output_type) {
    case "stream":
      return {
        kind: "text",
        text: stripAnsi(joinSource(output.text)),
        stream: output.name === "stderr" ? "stderr" : "stdout",
      };
    case "execute_result":
    case "display_data":
      return isObject(output.data) ? displayOutput(output.data) : null;
    case "error":
      return {
        kind: "error",
        name: [output.ename, output.evalue].filter(part => typeof part === "string").join(": "),
        text: Array.isArray(output.traceback)
          ? stripAnsi(output.traceback.filter(line => typeof line === "string").join("\n"))
          : "",
      };
    default:
      return null;
  }
}

function parseCell(cell: unknown, position: number): NotebookCell | null {
  if (!isObject(cell)) return null;
  const source = joinSource(cell.source);
  const id = typeof cell.id === "string" && cell.id !== "" ? cell.id : `cell-${position}`;
  switch (cell.cell_type) {
    case "markdown":
      return { id, kind: "markdown", source };
    case "raw":
      return { id, kind: "raw", source };
    case "code": {
      const outputs: NotebookOutput[] = [];
      for (const output of Array.isArray(cell.outputs) ? cell.outputs : []) {
        const body = parseOutput(output);
        if (body) outputs.push({ ...body, id: `${id}-output-${outputs.length}` });
      }
      return {
        id,
        kind: "code",
        source,
        executionCount: typeof cell.execution_count === "number" ? cell.execution_count : null,
        outputs,
      };
    }
    default:
      return null;
  }
}

/** The notebook in `text`, or `null` when it is not nbformat 4 JSON (truncated, malformed). */
export function parseNotebook(text: string): Notebook | null {
  let document: unknown;
  try {
    document = JSON.parse(text);
  } catch {
    return null;
  }
  if (!isObject(document) || !Array.isArray(document.cells)) return null;
  const metadata = isObject(document.metadata) ? document.metadata : {};
  const languageInfo = isObject(metadata.language_info) ? metadata.language_info : {};
  const kernelspec = isObject(metadata.kernelspec) ? metadata.kernelspec : {};
  const language =
    typeof languageInfo.name === "string"
      ? languageInfo.name
      : typeof kernelspec.language === "string"
        ? kernelspec.language
        : "python";
  return {
    language,
    cells: document.cells
      .map((cell, position) => parseCell(cell, position))
      .filter(cell => cell !== null),
  };
}
