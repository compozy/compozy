import { ArrowUpRight, CircleCheck } from "@compozy/ui/icons";
import { type ReactNode, useState } from "react";

import { Button, cn, Disclosure, Spinner, Textarea } from "@compozy/ui";

import { isAbsoluteMCPRedirectURL } from "../../lib/mcp-authorize-model";

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

/** Three plain steps that show where the person is in the sign-in. */
export function MCPSignInSteps({
  starting,
  onOpenAgain,
}: {
  starting: boolean;
  /** Reopens the sign-in page; absent until the page has an address. */
  onOpenAgain?: () => void;
}) {
  const steps: Step[] = [
    starting
      ? { status: "current", title: "Opening the sign-in page…" }
      : {
          status: "done",
          title: "Sign-in page opened",
          action: onOpenAgain ? (
            <Button
              type="button"
              size="xs"
              variant="link"
              className="h-auto px-0"
              onClick={onOpenAgain}
              data-testid="mcp-sign-in-open-again"
            >
              Open again
              <ArrowUpRight data-icon="inline-end" />
            </Button>
          ) : undefined,
        },
    {
      status: starting ? "todo" : "current",
      title: "Sign in and allow access",
      caption: starting ? undefined : "Finish it in your browser",
    },
    { status: "todo", title: "Back here by itself", caption: "No need to copy anything" },
  ];
  return (
    <ol className="flex flex-col" data-testid="mcp-sign-in-steps">
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
                aria-current={step.status === "current" ? "step" : undefined}
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

export interface MCPSignInPasteFallbackProps {
  /** The panel already takes a pasted address (manual sign-in, or a finish that failed). */
  accepting: boolean;
  /** The pasted address is being exchanged. */
  finishing: boolean;
  /** A manual sign-in is still being prepared. */
  preparing: boolean;
  /** Opening the fold asks for a sign-in that finishes by pasting the address. */
  onRequest: () => void;
  onSubmit: (value: string) => void;
}

/**
 * "Signing in on another computer?": folded until someone needs it. The browser's address after
 * sign-in, pasted whole, finishes the sign-in when it can't come back here by itself.
 */
export function MCPSignInPasteFallback({
  accepting,
  finishing,
  preparing,
  onRequest,
  onSubmit,
}: MCPSignInPasteFallbackProps) {
  const [open, setOpen] = useState(accepting || preparing);
  const [value, setValue] = useState("");
  const ready = accepting && isAbsoluteMCPRedirectURL(value);
  return (
    <Disclosure
      open={open}
      onOpenChange={next => {
        setOpen(next);
        if (next && !accepting && !preparing) onRequest();
      }}
      label="Signing in on another computer?"
      triggerProps={{ "data-testid": "mcp-sign-in-paste-toggle" }}
    >
      <div className="flex flex-col gap-2" data-testid="mcp-sign-in-paste">
        <p className="text-meta text-muted-foreground">
          When the browser lands on a page that doesn't load, copy its address and paste it here.
        </p>
        <Textarea
          variant="mono"
          rows={2}
          aria-label="Address after sign-in"
          placeholder="http://127.0.0.1:…/callback?code=…"
          value={value}
          disabled={!accepting || finishing}
          onChange={event => setValue(event.target.value)}
          className="min-h-0 resize-none text-meta"
          data-testid="mcp-sign-in-paste-input"
        />
        <Button
          type="button"
          size="sm"
          disabled={!ready || finishing}
          className="self-end"
          onClick={() => onSubmit(value)}
          data-testid="mcp-sign-in-paste-submit"
        >
          {finishing || preparing ? <Spinner className="size-3.5" /> : null}
          {finishing ? "Finishing…" : "Finish sign-in"}
        </Button>
      </div>
    </Disclosure>
  );
}
