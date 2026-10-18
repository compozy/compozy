import { readEditToolInput } from "../../lib/tool-edit-diff";
import { alignLines, splitLines } from "../../lib/tool-diff-stat";
import type { UIMessage } from "../../types";
import { GenericContent } from "./generic-content";
import { ToolDiffBody } from "./tool-diff-body";

/**
 * Edit detail as a diff card: the replaced text aligned against its
 * replacement line by line, unchanged lines as context between the removed
 * and added ones.
 */
export function EditContent({ message }: { message: UIMessage }) {
  const { filePath, oldStr, newStr } = readEditToolInput(message);

  if (!filePath && !oldStr && !newStr) {
    return <GenericContent message={message} />;
  }

  return (
    <ToolDiffBody
      testId="edit-content"
      {...(filePath ? { path: filePath } : {})}
      lines={alignLines(splitLines(oldStr), splitLines(newStr))}
    />
  );
}
