import type { ComponentProps } from "react";

import { CodeBlock, CopyIconButton, Eyebrow, cn } from "@compozy/ui";

import { triggerWebhookCurl } from "../../lib/trigger-sentence";

/**
 * Local delivery path for a webhook trigger — the POST that always answers on
 * this daemon. Public reachability is a separate fact and lives in the rail.
 */
type TriggerWebhookEndpointProps = Omit<ComponentProps<"div">, "children"> & { path: string };

export function TriggerWebhookEndpoint({ path, className, ...props }: TriggerWebhookEndpointProps) {
  return (
    <div
      className={cn("mt-2.5 flex flex-col gap-2", className)}
      data-testid="trigger-webhook-endpoint"
      {...props}
    >
      <div className="flex items-center gap-2 rounded-sm border border-line-soft bg-rail px-2.5 py-2">
        <Eyebrow className="rounded-xs bg-canvas-soft px-1.5 py-0.5 text-subtle">POST</Eyebrow>
        <span className="min-w-0 flex-1 truncate font-mono text-form-hint text-fg">{path}</span>
        <CopyIconButton
          copiedLabel="Webhook path copied"
          copyLabel="Copy webhook path"
          value={path}
        />
      </div>
      {/* No language: syntax colour would read as state on a page where colour means state. */}
      <CodeBlock
        className="bg-rail"
        code={triggerWebhookCurl(path)}
        copyable={false}
        density="compact"
      />
    </div>
  );
}
