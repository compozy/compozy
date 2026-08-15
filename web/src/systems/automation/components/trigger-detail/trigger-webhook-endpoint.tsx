import { CodeBlock, CopyIconButton } from "@compozy/ui";

import { triggerWebhookCurl } from "../../lib/trigger-sentence";

/**
 * Local delivery path for a webhook trigger — the POST that always answers on
 * this daemon. Public reachability is a separate fact and lives in the rail.
 */
export function TriggerWebhookEndpoint({ path }: { path: string }) {
  return (
    <div className="mt-2.5 flex flex-col gap-2" data-testid="trigger-webhook-endpoint">
      <div className="flex items-center gap-2 rounded-sm border border-line-soft bg-rail px-2.5 py-2">
        <span className="rounded-xs bg-success-tint px-1.5 py-0.5 font-mono text-badge font-semibold text-success">
          POST
        </span>
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
        showPrompt={false}
      />
    </div>
  );
}
