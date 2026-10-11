import type { UIMessage } from "../types";

export interface EditToolInput {
  filePath: string;
  oldStr: string;
  newStr: string;
}

function optionalText(value: unknown): string {
  return value != null ? String(value) : "";
}

/** Reads the edited path and the replaced/replacement text from an Edit tool call. */
export function readEditToolInput(message: UIMessage): EditToolInput {
  const input = message.toolInput;
  return {
    filePath: String(input?.file_path ?? input?.filePath ?? message.toolResult?.filePath ?? ""),
    oldStr: optionalText(input?.old_string),
    newStr: optionalText(input?.new_string),
  };
}
