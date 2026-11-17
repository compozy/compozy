import { CircleAlert, CircleCheck, KeyRound, Lock, ShieldCheck } from "@compozy/ui/icons";

import { Button, CopyIconButton, PopoverPanel } from "@compozy/ui";

import { MCPSignInPasteFallback, MCPSignInSteps } from "./mcp-sign-in-steps";

/** The sign-in as the person sees it; `mcp-authorize-store` phases map onto these one to one. */
export type MCPSignInPhase =
  | "scopes"
  | "starting"
  | "waiting"
  | "paste"
  | "finishing"
  | "connected"
  | "failed";

export interface MCPSignInPanelProps {
  server: string;
  phase: MCPSignInPhase;
  /** Where the sign-in happens, shown under the title. */
  host?: string;
  /** The sign-in page's address, once CompozyOS has one. */
  url?: string;
  /** The current attempt finishes by pasting the address (signing in on another computer). */
  manual?: boolean;
  /** A failed attempt still has a sign-in page, so a pasted address can finish it. */
  canFinishByPaste?: boolean;
  error?: string | null;
  /** Access asked for beyond what the server was given last time. */
  scopes?: readonly string[];
  /** Tools the server offers once connected; omitted when unknown. */
  toolCount?: number;
  /** Settings applies a sign-in at once; a chat picks it up on its next message. */
  appliesOn?: "now" | "next-message";
  onClose: () => void;
  onContinue: () => void;
  onRetry: () => void;
  onOpenAgain: () => void;
  onRequestPaste: () => void;
  onSubmitPaste: (value: string) => void;
}

function CopyLink({ url }: { url?: string }) {
  if (!url) return null;
  return (
    <CopyIconButton
      value={url}
      size="icon-sm"
      copyLabel="Copy sign-in link"
      copiedToastLabel="Sign-in link copied"
      className="text-muted-foreground"
      data-testid="mcp-sign-in-copy-link"
    />
  );
}

function connectedLine(server: string, toolCount: number | undefined, appliesOn: string) {
  const tools = toolCount && toolCount > 0 ? `${toolCount} tools` : "Its tools";
  return appliesOn === "now"
    ? `${tools} from ${server} are ready to use.`
    : `${tools} from ${server} join this chat on your next message.`;
}

/**
 * The whole sign-in in a panel beside the server: the browser opens on its own, three plain steps
 * show where the person is, and the paste-the-address fallback stays folded until someone needs
 * it. A failure says nothing changed; a request for more access is confirmed before anything opens.
 */
export function MCPSignInPanel({
  server,
  phase,
  host,
  url,
  manual = false,
  canFinishByPaste = false,
  error,
  scopes = [],
  toolCount,
  appliesOn = "now",
  onClose,
  onContinue,
  onRetry,
  onOpenAgain,
  onRequestPaste,
  onSubmitPaste,
}: MCPSignInPanelProps) {
  if (phase === "connected") {
    return (
      <PopoverPanel
        data-testid="mcp-sign-in-panel"
        data-phase={phase}
        icon={CircleCheck}
        tone="success"
        title={`Connected to ${server}`}
        subtitle={host}
        onClose={onClose}
        footer={
          <Button
            type="button"
            size="sm"
            variant="secondary"
            className="ml-auto"
            onClick={onClose}
            data-testid="mcp-sign-in-done"
          >
            Done
          </Button>
        }
      >
        <p className="text-small-body text-fg" role="status">
          {connectedLine(server, toolCount, appliesOn)}
        </p>
      </PopoverPanel>
    );
  }
  if (phase === "failed") {
    return (
      <PopoverPanel
        data-testid="mcp-sign-in-panel"
        data-phase={phase}
        icon={CircleAlert}
        tone="destructive"
        title="Sign-in didn't finish"
        subtitle={host}
        onClose={onClose}
        aside={
          <MCPSignInPasteFallback
            accepting={canFinishByPaste}
            finishing={false}
            preparing={false}
            onRequest={onRequestPaste}
            onSubmit={onSubmitPaste}
          />
        }
        footer={
          <>
            <Button type="button" size="sm" variant="ghost" className="ml-auto" onClick={onClose}>
              Cancel
            </Button>
            <Button type="button" size="sm" onClick={onRetry} data-testid="mcp-sign-in-retry">
              Try again
            </Button>
          </>
        }
      >
        <p className="text-small-body text-fg" role="alert">
          {error ?? "Something went wrong in the browser."}
        </p>
        <p className="text-meta text-muted-foreground">
          Nothing changed: {server} keeps its previous sign-in.
        </p>
      </PopoverPanel>
    );
  }
  if (phase === "scopes") {
    return (
      <PopoverPanel
        data-testid="mcp-sign-in-panel"
        data-phase={phase}
        icon={ShieldCheck}
        tone="warning"
        title={`${server} asks for more access`}
        subtitle={host}
        onClose={onClose}
        footer={
          <>
            <Button type="button" size="sm" variant="ghost" className="ml-auto" onClick={onClose}>
              Not now
            </Button>
            <Button
              type="button"
              size="sm"
              disabled={scopes.length === 0}
              onClick={onContinue}
              data-testid="mcp-sign-in-continue"
            >
              Continue
            </Button>
          </>
        }
      >
        <p className="text-meta text-muted-foreground">
          {server} wants more access than you gave it last time. You'll confirm in your browser.
        </p>
        <ul
          aria-label="More access"
          className="flex flex-col overflow-hidden rounded-md bg-sunken shadow-hairline-inset"
        >
          {scopes.map(scope => (
            <li
              key={scope}
              className="flex items-start gap-2.5 border-b border-line-soft px-2.5 py-2 last:border-b-0"
            >
              <Lock aria-hidden="true" className="mt-px size-3.5 shrink-0 text-faint" />
              <span className="min-w-0 flex-1 font-mono text-mono-id wrap-anywhere text-fg">
                {scope}
              </span>
            </li>
          ))}
        </ul>
      </PopoverPanel>
    );
  }
  const starting = phase === "starting";
  const stage = starting ? "starting" : phase === "finishing" ? "finishing" : "waiting";
  return (
    <PopoverPanel
      data-testid="mcp-sign-in-panel"
      data-phase={phase}
      icon={KeyRound}
      title={`Sign in to ${server}`}
      subtitle={host}
      onClose={onClose}
      aside={
        starting && !manual ? undefined : (
          <MCPSignInPasteFallback
            accepting={phase === "paste" || phase === "finishing"}
            finishing={phase === "finishing"}
            preparing={starting && manual}
            onRequest={onRequestPaste}
            onSubmit={onSubmitPaste}
          />
        )
      }
      footer={
        <>
          <CopyLink url={url} />
          <Button type="button" size="sm" variant="ghost" className="ml-auto" onClick={onClose}>
            Cancel
          </Button>
        </>
      }
    >
      <MCPSignInSteps
        stage={stage}
        variant={manual ? "manual" : "automatic"}
        onOpenAgain={url ? onOpenAgain : undefined}
      />
    </PopoverPanel>
  );
}
