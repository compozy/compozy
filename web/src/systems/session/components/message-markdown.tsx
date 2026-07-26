import { memo } from "react";

import { StreamMarkdown } from "@agh/ui";

export interface MessageMarkdownProps {
  content: string;
  streaming?: boolean;
}

export const MessageMarkdown = memo(
  function MessageMarkdown({ content, streaming = false }: MessageMarkdownProps) {
    return <StreamMarkdown streaming={streaming}>{content}</StreamMarkdown>;
  },
  (prev, next) => prev.content === next.content && prev.streaming === next.streaming
);
