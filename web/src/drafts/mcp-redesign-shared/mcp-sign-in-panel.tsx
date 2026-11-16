/**
 * Proposed inline sign-in: the whole OAuth flow in a panel beside the server row (nested popover),
 * replacing the "Authorize <server>" modal. Three plain steps show where the person is; the
 * paste-the-address fallback stays folded in a ruled section until someone needs it.
 *
 * Phases map 1:1 to `mcp-authorize-store` (`beginning` → starting, `waiting`, `manual` → paste,
 * `exchanging` → finishing, `confirmed` → connected, `failed`, `reviewing_scopes` → scopes).
 */
import {
  ArrowUpRight,
  CircleAlert,
  CircleCheck,
  KeyRound,
  Lock,
  ShieldCheck,
} from "@compozy/ui/icons";
import type { ReactNode } from "react";

import { Button, CopyIconButton, cn, Disclosure, Spinner, Textarea } from "@compozy/ui";

import { MCP_PANEL_CLASS, MCPPanel } from "./mcp-panel";

export { MCP_PANEL_CLASS };

export type MCPSignInPhase =
  | "starting"
  | "waiting"
  | "paste"
  | "finishing"
  | "connected"
  | "failed"
  | "scopes";

export interface MCPSignInScope {
  id: string;
  label: string;
}

export interface MCPSignInPanelProps {
  server: string;
  /** Where the sign-in happens, shown under the title. */
  host?: string;
  phase: MCPSignInPhase;
  error?: string;
  scopes?: readonly MCPSignInScope[];
  toolCount?: number;
  /** A few tool names to preview once connected. */
  toolPreview?: readonly string[];
  appliesOn?: "next-message" | "now";
  pasted?: string;
  onClose?: () => void;
}

type StepStatus = "done" | "current" | "todo";

interface Step {
  status: StepStatus;
  title: string;
  caption?: string;
  action?: ReactNode;
}

function StepIndicator({ status }: { status: StepStatus }) {
  if (status === "done") {
    return <CircleCheck aria-hidden="true" className="size-4.5 text-success" />;
  }
  if (status === "current") {
    return (
      <span className="flex size-4.5 items-center justify-center rounded-full bg-surface-2 text-fg">
        <Spinner className="size-3" />
      </span>
    );
  }
  return (
    <span className="flex size-4.5 items-center justify-center">
      <span className="size-2 rounded-full bg-faint/50" />
    </span>
  );
}

function Steps({ steps }: { steps: readonly Step[] }) {
  return (
    <ol className="flex flex-col">
      {steps.map((step, index) => (
        <li key={step.title} className="relative flex gap-3 pb-3 last:pb-0">
          {index < steps.length - 1 ? (
            <span
              aria-hidden="true"
              className="absolute top-5.5 bottom-1 left-[8.5px] w-px bg-line"
            />
          ) : null}
          <span className="relative mt-px shrink-0">
            <StepIndicator status={step.status} />
          </span>
          <div className="flex min-w-0 flex-1 flex-col gap-0.5">
            <span className="flex items-center justify-between gap-2">
              <span
                className={cn(
                  "text-small-body",
                  step.status === "current" && "font-medium text-fg-strong",
                  step.status === "done" && "text-muted-foreground",
                  step.status === "todo" && "text-faint"
                )}
              >
                {step.title}
              </span>
              {step.action}
            </span>
            {step.caption ? <span className="text-meta text-faint">{step.caption}</span> : null}
          </div>
        </li>
      ))}
    </ol>
  );
}

function stepsFor(phase: MCPSignInPhase): Step[] {
  const opened: Step =
    phase === "starting"
      ? { status: "current", title: "Opening the sign-in page…" }
      : {
          status: "done",
          title: "Sign-in page opened",
          action: (
            <Button type="button" size="xs" variant="link" className="h-auto px-0">
              Open again
              <ArrowUpRight data-icon="inline-end" />
            </Button>
          ),
        };
  return [
    opened,
    {
      status: phase === "starting" ? "todo" : "current",
      title: "Sign in and allow access",
      caption: phase === "starting" ? undefined : "Finish it in your browser",
    },
    { status: "todo", title: "Back here by itself", caption: "No need to copy anything" },
  ];
}

function PasteFallback({
  open,
  value,
  finishing,
}: {
  open: boolean;
  value?: string;
  finishing: boolean;
}) {
  return (
    <Disclosure defaultOpen={open} label="Signing in on another computer?">
      <div className="flex flex-col gap-2">
        <p className="text-meta text-muted-foreground">
          When the browser lands on a page that doesn't load, copy its address and paste it here.
        </p>
        <Textarea
          variant="mono"
          rows={2}
          aria-label="Address after sign-in"
          placeholder="http://127.0.0.1:…/callback?code=…"
          defaultValue={value}
          className="min-h-0 resize-none text-meta"
        />
        <Button type="button" size="sm" disabled={!value || finishing} className="self-end">
          {finishing ? <Spinner className="size-3.5" /> : null}
          {finishing ? "Finishing…" : "Finish sign-in"}
        </Button>
      </div>
    </Disclosure>
  );
}

function CopyLink() {
  return (
    <CopyIconButton
      value="https://dashboard.branas.dev/oauth/authorize?client_id=compozy"
      size="icon-sm"
      copyLabel="Copy sign-in link"
      copiedToastLabel="Sign-in link copied"
      className="text-muted-foreground"
    />
  );
}

function Connected({
  server,
  toolCount,
  toolPreview = [],
  appliesOn,
}: Pick<MCPSignInPanelProps, "server" | "toolCount" | "toolPreview"> & {
  appliesOn: "next-message" | "now";
}) {
  return (
    <>
      <p className="text-small-body text-fg">
        {appliesOn === "now"
          ? `${toolCount ?? "Its"} tools from ${server} are ready to use.`
          : `${toolCount ?? "Its"} tools from ${server} join this chat on your next message.`}
      </p>
      {toolPreview.length > 0 ? (
        <ul className="flex flex-wrap gap-1">
          {toolPreview.map(tool => (
            <li key={tool} className="rounded-sm bg-surface-2 px-1.5 py-0.5 text-meta text-muted-foreground">
              {tool}
            </li>
          ))}
          {toolCount && toolCount > toolPreview.length ? (
            <li className="px-1 py-0.5 text-meta text-faint">+{toolCount - toolPreview.length} more</li>
          ) : null}
        </ul>
      ) : null}
    </>
  );
}

function Scopes({ server, scopes }: { server: string; scopes: readonly MCPSignInScope[] }) {
  return (
    <>
      <p className="text-meta text-muted-foreground">
        {server} wants more access than you gave it last time. You'll confirm in your browser.
      </p>
      <ul className="flex flex-col overflow-hidden rounded-md bg-sunken shadow-hairline-inset">
        {scopes.map(scope => (
          <li
            key={scope.id}
            className="flex items-center gap-2.5 border-b border-line-soft px-2.5 py-2 last:border-b-0"
          >
            <Lock aria-hidden="true" className="size-3.5 shrink-0 text-faint" />
            <span className="min-w-0 flex-1 truncate text-small-body text-fg">{scope.label}</span>
            <span className="shrink-0 font-mono text-mono-id text-faint">{scope.id}</span>
          </li>
        ))}
      </ul>
    </>
  );
}

export function MCPSignInPanel({
  server,
  host = "dashboard.branas.dev",
  phase,
  error,
  scopes = [],
  toolCount,
  toolPreview,
  appliesOn = "next-message",
  pasted,
  onClose,
}: MCPSignInPanelProps) {
  const flow = phase === "starting" || phase === "waiting" || phase === "paste" || phase === "finishing";
  if (phase === "connected") {
    return (
      <MCPPanel
        testId="mcp-sign-in-panel"
        icon={CircleCheck}
        tone="success"
        title={`Connected to ${server}`}
        subtitle={host}
        onClose={onClose}
        footer={
          <Button type="button" size="sm" variant="secondary" className="ml-auto" onClick={onClose}>
            Done
          </Button>
        }
      >
        <Connected server={server} toolCount={toolCount} toolPreview={toolPreview} appliesOn={appliesOn} />
      </MCPPanel>
    );
  }
  if (phase === "failed") {
    return (
      <MCPPanel
        testId="mcp-sign-in-panel"
        icon={CircleAlert}
        tone="destructive"
        title="Sign-in didn't finish"
        subtitle={host}
        onClose={onClose}
        aside={<PasteFallback open={false} finishing={false} />}
        footer={
          <>
            <Button type="button" size="sm" variant="ghost" className="ml-auto" onClick={onClose}>
              Cancel
            </Button>
            <Button type="button" size="sm">
              Try again
            </Button>
          </>
        }
      >
        <p className="text-small-body text-fg">
          {error ?? "Something went wrong in the browser."}
        </p>
        <p className="text-meta text-muted-foreground">
          Nothing changed: {server} keeps its previous sign-in.
        </p>
      </MCPPanel>
    );
  }
  if (phase === "scopes") {
    return (
      <MCPPanel
        testId="mcp-sign-in-panel"
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
            <Button type="button" size="sm">
              Continue
            </Button>
          </>
        }
      >
        <Scopes server={server} scopes={scopes} />
      </MCPPanel>
    );
  }
  return (
    <MCPPanel
      testId="mcp-sign-in-panel"
      icon={KeyRound}
      title={`Sign in to ${server}`}
      subtitle={host}
      onClose={onClose}
      aside={
        flow && phase !== "starting" ? (
          <PasteFallback open={phase !== "waiting"} value={pasted} finishing={phase === "finishing"} />
        ) : undefined
      }
      footer={
        <>
          <CopyLink />
          <Button type="button" size="sm" variant="ghost" className="ml-auto" onClick={onClose}>
            Cancel
          </Button>
        </>
      }
    >
      <Steps steps={stepsFor(phase)} />
    </MCPPanel>
  );
}
