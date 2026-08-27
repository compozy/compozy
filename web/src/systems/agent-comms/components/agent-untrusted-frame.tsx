/**
 * A bounded frame for text an agent wrote.
 *
 * Message bodies and agent descriptions are model output. They can contain
 * anything — including text shaped like an instruction to the operator, or to
 * whatever reads the screen next. Three properties make that safe:
 *
 * 1. **Stamped.** The frame names who wrote it and says plainly that it is not
 *    the operator, so a message can never be mistaken for the system talking.
 * 2. **Bounded.** A dashed hairline marks where untrusted text starts and stops,
 *    so it cannot visually merge into surrounding chrome.
 * 3. **Inert.** The body is rendered as plain text — never Markdown, never HTML,
 *    never a link. An embedded command arrives as characters on a screen and
 *    nothing more, and nothing inside a frame can approve a pending permission.
 *
 * Deliberately not a Markdown renderer: this is the one place in the app where
 * *less* rendering is the feature.
 */
import { UntrustedFrame } from "@compozy/ui";

export interface AgentUntrustedFrameProps extends Omit<React.ComponentProps<"aside">, "children"> {
  /** Who wrote it — an agent name when known, else the session identity. */
  authorLabel: string;
  /** The session the text came from, shown beside the author when known. */
  sourceId?: string | null;
  children: string;
}

export function AgentUntrustedFrame({
  authorLabel,
  sourceId,
  children,
  ...props
}: AgentUntrustedFrameProps) {
  const stamp = `from agent ${authorLabel}${sourceId ? ` (${sourceId})` : ""} — not the operator`;
  return (
    <UntrustedFrame stamp={stamp} data-slot="agent-untrusted-frame" {...props}>
      {children}
    </UntrustedFrame>
  );
}
