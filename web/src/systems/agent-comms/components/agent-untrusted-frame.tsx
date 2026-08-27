/**
 * Domain wrapper around the shared untrusted frame.
 *
 * The stamp names the agent and says the text is not the operator. The session
 * id stays off the stamp — it is an identifier, not a speaker.
 */
import { UntrustedFrame } from "@compozy/ui";

export interface AgentUntrustedFrameProps extends Omit<React.ComponentProps<"aside">, "children"> {
  /** Who wrote it: an agent name when known, otherwise a plain fallback. */
  authorLabel: string;
  children: string;
}

export function AgentUntrustedFrame({ authorLabel, children, ...props }: AgentUntrustedFrameProps) {
  return (
    <UntrustedFrame
      {...props}
      data-domain="agent-untrusted-frame"
      stamp={`from agent ${authorLabel} — not the operator`}
    >
      {children}
    </UntrustedFrame>
  );
}
