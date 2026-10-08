import type { ComponentProps } from "react";

import { CodeBlock, CopyIconButton, Disclosure, Eyebrow, cn } from "@compozy/ui";

import { triggerWebhookCurl } from "../../lib/automation-rule";

/**
 * Local delivery path for a webhook automation — the POST that always answers
 * on this daemon, with the signed example request one disclosure away. Public
 * reachability is a separate fact and lives in the rail.
 */
type AutomationWebhookEndpointProps = Omit<ComponentProps<"div">, "children"> & { path: string };

export function AutomationWebhookEndpoint({
  path,
  className,
  ...props
}: AutomationWebhookEndpointProps) {
  return (
    <div
      className={cn("mt-2.5 flex flex-col gap-2", className)}
      data-testid="automation-webhook-endpoint"
      {...props}
    >
      <div className="flex items-center gap-2 rounded-sm border border-line-soft bg-rail px-2.5 py-2">
        <Eyebrow className="rounded-xs bg-surface-2 px-1.5 py-0.5 text-subtle">POST</Eyebrow>
        <span className="min-w-0 flex-1 truncate font-mono text-form-hint text-fg">{path}</span>
        <CopyIconButton
          copiedLabel="Webhook path copied"
          copyLabel="Copy webhook path"
          value={path}
        />
      </div>
      <Disclosure
        label="Show example request"
        size="sm"
        triggerProps={{ "data-testid": "automation-webhook-example-toggle" }}
      >
        {/* No language: syntax colour would read as state on a page where colour means state. */}
        <CodeBlock
          className="bg-rail"
          code={triggerWebhookCurl(path)}
          copyable={false}
          density="compact"
        />
      </Disclosure>
    </div>
  );
}
