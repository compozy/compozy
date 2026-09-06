import { memo } from "react";

import { StreamMarkdown } from "@compozy/ui";

import { usePrefersReducedMotion } from "@/components/assistant-ui/hooks/use-prefers-reduced-motion";
import { useSmoothStreamedText } from "../hooks/use-smooth-streamed-text";
import { useSmoothStreamingPreference } from "../hooks/use-smooth-streaming-preference";
import { useThrottledStreamingValue } from "../hooks/use-throttled-streaming-value";
import { composeStreamingDisplay } from "../lib/session-smooth-reveal";

export interface MessageMarkdownProps {
  content: string;
  streaming?: boolean;
  /**
   * Assistant prose only (ADR-008): reveal the streamed text smoothly, snap on
   * completion, and hand an open code block to the throttled highlighter
   * cadence. Off for reasoning, tool output, and everything that is not the
   * answer.
   */
  reveal?: boolean;
}

// The reveal is presentation only: reduced motion and the client-local
// "Smooth streaming" preference both render chunks as they arrive.
function RevealedMarkdown({ content, streaming }: { content: string; streaming: boolean }) {
  const reducedMotion = usePrefersReducedMotion();
  const preference = useSmoothStreamingPreference();
  const animate = streaming && preference.enabled && !reducedMotion;
  const revealed = useSmoothStreamedText(content, animate);
  const highlighted = useThrottledStreamingValue(revealed, streaming);
  const display = streaming ? composeStreamingDisplay(revealed, highlighted) : content;
  return (
    <StreamMarkdown
      streaming={streaming}
      data-reveal={animate ? "smooth" : "direct"}
      data-testid="message-markdown"
    >
      {display}
    </StreamMarkdown>
  );
}

export const MessageMarkdown = memo(
  function MessageMarkdown({ content, streaming = false, reveal = false }: MessageMarkdownProps) {
    if (reveal) {
      return <RevealedMarkdown content={content} streaming={streaming} />;
    }
    return <StreamMarkdown streaming={streaming}>{content}</StreamMarkdown>;
  },
  (prev, next) =>
    prev.content === next.content &&
    prev.streaming === next.streaming &&
    prev.reveal === next.reveal
);
