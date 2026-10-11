import { alignLines, splitLines } from "../../lib/tool-diff-stat";
import type { UIMessage } from "../../types";
import { GenericContent } from "./generic-content";
import { ToolDiffBody } from "./tool-diff-body";

/** Write detail as a diff card: the written content as added lines under its path. */
export function WriteContent({ message }: { message: UIMessage }) {
  const filePath = String(
    message.toolInput?.file_path ??
      message.toolInput?.filePath ??
      message.toolResult?.filePath ??
      ""
  );
  const content = String(message.toolInput?.content ?? message.toolResult?.content ?? "");

  if (!filePath && !content) {
    return <GenericContent message={message} />;
  }

  return (
    <ToolDiffBody
      testId="write-content"
      {...(filePath ? { path: filePath } : {})}
      lines={alignLines([], splitLines(content))}
    />
  );
}
