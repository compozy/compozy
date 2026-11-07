import { memo, type ImgHTMLAttributes, type ReactNode } from "react";

import {
  StreamMarkdown,
  type StreamMarkdownInlineCodeDecorator,
  type StreamMarkdownProps,
  type StreamMarkdownSlots,
} from "@compozy/ui";

import { usePrefersReducedMotion } from "@/components/assistant-ui/hooks/use-prefers-reduced-motion";
import { useFilePathLinks } from "../hooks/use-file-path-links";
import { FilePathLinksContext } from "../lib/file-path-links-context";
import { FilePathAnchor, FilePathInlineCode, FilePathLocalLink } from "./file-path-link";
import { useSmoothStreamedText } from "../hooks/use-smooth-streamed-text";
import { useSmoothStreamingPreference } from "../hooks/use-smooth-streaming-preference";
import { useThrottledStreamingValue } from "../hooks/use-throttled-streaming-value";
import { composeStreamingDisplay } from "../lib/session-smooth-reveal";

function MessageImageReference({
  src,
  alt,
}: ImgHTMLAttributes<HTMLImageElement> | Record<string, unknown>) {
  const pending = src === "streamdown:incomplete-image";
  const path = typeof src === "string" && !pending ? src : "";
  const label = typeof alt === "string" && alt ? alt : "image";
  return (
    <span title={path} aria-busy={pending || undefined}>
      [image: {label}]
      {path ? (
        <>
          {" "}
          <code>{path}</code>
        </>
      ) : null}
    </span>
  );
}

const MESSAGE_MARKDOWN_COMPONENTS = { img: MessageImageReference };

const decorateFilePathInlineCode: StreamMarkdownInlineCodeDecorator = code => (
  <FilePathInlineCode text={code.text} element={code.element} />
);

type FilePathMarkdownProps = Pick<
  StreamMarkdownProps,
  "components" | "inlineCodeDecorator" | "localLinkComponent"
>;

const PLAIN_MARKDOWN_PROPS: FilePathMarkdownProps = { components: MESSAGE_MARKDOWN_COMPONENTS };
// This session's paths: a click opens the viewer, Cmd/Ctrl-click the desktop editor (US-005).
const FILE_PATH_MARKDOWN_PROPS: FilePathMarkdownProps = {
  components: { ...MESSAGE_MARKDOWN_COMPONENTS, a: FilePathAnchor },
  inlineCodeDecorator: decorateFilePathInlineCode,
  localLinkComponent: FilePathLocalLink,
};

/** Path links apply only when this session has an action for them; otherwise plain markdown. */
function FilePathLinksScope({
  children,
}: {
  children: (markdownProps: FilePathMarkdownProps) => ReactNode;
}) {
  const links = useFilePathLinks();
  return (
    <FilePathLinksContext value={links}>
      {children(links ? FILE_PATH_MARKDOWN_PROPS : PLAIN_MARKDOWN_PROPS)}
    </FilePathLinksContext>
  );
}

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
  /** Prose density; the reasoning panel uses `"relaxed"` to keep small headings with prose breaks. */
  compact?: StreamMarkdownProps["compact"];
  /** Inline nodes the content places with `composeMarkdownSlots` (transcript chips). */
  slots?: StreamMarkdownSlots;
  /** Operator-typed prose: authored line breaks and safe external links (`StreamMarkdown`). */
  authored?: boolean;
  /**
   * This session's assistant-authored prose: file paths in inline code and links open in the
   * session's viewer (and the editor on desktop). User turns and other sessions' reply cards
   * leave it off.
   */
  openPaths?: boolean;
}

// The reveal is presentation only: reduced motion and the client-local
// "Smooth streaming" preference both render chunks as they arrive.
function RevealedMarkdown({
  content,
  streaming,
  compact,
  markdownProps,
}: Required<Pick<MessageMarkdownProps, "content" | "streaming" | "compact">> & {
  markdownProps: FilePathMarkdownProps;
}) {
  const reducedMotion = usePrefersReducedMotion();
  const preference = useSmoothStreamingPreference();
  const animate = streaming && preference.enabled && !reducedMotion;
  const revealed = useSmoothStreamedText(content, animate);
  const highlighted = useThrottledStreamingValue(revealed, streaming);
  const display = streaming ? composeStreamingDisplay(revealed, highlighted) : content;
  return (
    <StreamMarkdown
      streaming={streaming}
      {...markdownProps}
      compact={compact}
      data-reveal={animate ? "smooth" : "direct"}
      data-testid="message-markdown"
    >
      {display}
    </StreamMarkdown>
  );
}

export const MessageMarkdown = memo(
  function MessageMarkdown({
    content,
    streaming = false,
    reveal = false,
    compact = false,
    slots,
    authored = false,
    openPaths = false,
  }: MessageMarkdownProps) {
    const render = (markdownProps: FilePathMarkdownProps) =>
      reveal ? (
        <RevealedMarkdown
          content={content}
          streaming={streaming}
          compact={compact}
          markdownProps={markdownProps}
        />
      ) : (
        <StreamMarkdown
          streaming={streaming}
          compact={compact}
          slots={slots}
          authored={authored}
          {...markdownProps}
        >
          {content}
        </StreamMarkdown>
      );
    return openPaths ? (
      <FilePathLinksScope>{render}</FilePathLinksScope>
    ) : (
      render(PLAIN_MARKDOWN_PROPS)
    );
  },
  (prev, next) =>
    prev.content === next.content &&
    prev.streaming === next.streaming &&
    prev.reveal === next.reveal &&
    prev.compact === next.compact &&
    prev.slots === next.slots &&
    prev.authored === next.authored &&
    prev.openPaths === next.openPaths
);
