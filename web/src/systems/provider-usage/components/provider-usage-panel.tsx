import { RefreshCw, Unplug } from "lucide-react";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  cn,
  Empty,
  Icon,
  PopoverHeader,
  PopoverTitle,
  ScrollArea,
  SkeletonRows,
  Spinner,
} from "@compozy/ui";

import { useUsageClock } from "../hooks/use-usage-clock";
import { formatUpdated } from "../lib/format";
import type { ProviderUsageRowAction } from "../lib/status";
import {
  partitionProviderUsageRows,
  PROVIDER_USAGE_TITLE,
  type ProviderUsageViewModel,
} from "../lib/view-model";

import { ProviderUsageQuietGroup } from "./provider-usage-quiet-group";
import { ProviderUsageRow } from "./provider-usage-row";

export interface ProviderUsagePanelProps {
  viewModel: ProviderUsageViewModel | null;
  /** First read in flight with nothing cached. */
  loading?: boolean;
  /** The daemon cannot be reached: cached rows stay, Refresh and row actions go. */
  daemonUnavailable?: boolean;
  /** A read failure other than reachability, shown when there is nothing cached. */
  errorMessage?: string | null;
  /**
   * `popover` titles through the popover's own title; `window` is the Usage
   * app, whose window chrome already carries the title, so the panel keeps
   * only the freshness line and Refresh.
   */
  chrome?: "popover" | "window";
  /** Defaults to "Provider usage"; the Usage app passes "Usage". */
  title?: string;
  /** Header Refresh (explicit intent). Absent hides the button. */
  onRefresh?: () => void;
  /** An explicit header refresh is in flight. */
  refreshing?: boolean;
  onAction?: (providerId: string, action: ProviderUsageRowAction) => void;
  pendingProviderIds?: ReadonlySet<string>;
  isMacHost: boolean;
  remoteClient?: boolean;
  /** Footer: opens the Usage app (the OS layer owns windows). */
  onOpenUsage?: () => void;
  /** Footer and turned-off state: opens the provider usage settings. */
  onOpenSettings?: () => void;
  className?: string;
}

const NO_PENDING: ReadonlySet<string> = new Set();

function PanelBody({ props, now }: { props: ProviderUsagePanelProps; now: number }) {
  const { viewModel, loading, errorMessage, chrome = "popover" } = props;
  if (!viewModel) {
    if (loading) return <SkeletonRows count={2} className="gap-4 px-2 py-2" aria-busy="true" />;
    if (errorMessage) {
      return (
        <Empty size="compact" title="Couldn't load provider usage" description={errorMessage} />
      );
    }
    return null;
  }
  if (!viewModel.enabled) {
    return (
      <Empty
        size="compact"
        title="Provider usage is turned off"
        action={
          props.onOpenSettings ? (
            <Button variant="outline" size="sm" onClick={props.onOpenSettings}>
              Open settings
            </Button>
          ) : undefined
        }
      />
    );
  }
  if (viewModel.rows.length === 0) {
    return (
      <Empty
        size="compact"
        title="No providers to show"
        description="Configure a provider to see its usage here."
      />
    );
  }
  const onAction = props.daemonUnavailable ? undefined : props.onAction;
  const pending = props.pendingProviderIds ?? NO_PENDING;
  const { measured, quiet } = partitionProviderUsageRows(viewModel.rows);
  return (
    <ScrollArea
      className={cn("min-h-0", chrome === "popover" ? "max-h-[min(60vh,28rem)]" : "flex-1")}
      viewportClassName={chrome === "popover" ? "max-h-[min(60vh,28rem)]" : undefined}
    >
      <ul aria-label="Providers" className="flex flex-col gap-0.5">
        {measured.map(row => (
          <li key={row.providerId}>
            <ProviderUsageRow
              row={row}
              now={now}
              isMacHost={props.isMacHost}
              remoteClient={props.remoteClient}
              onAction={onAction}
              pending={pending.has(row.providerId)}
            />
          </li>
        ))}
      </ul>
      <ProviderUsageQuietGroup
        rows={quiet}
        now={now}
        isMacHost={props.isMacHost}
        remoteClient={props.remoteClient}
        onAction={onAction}
        pendingProviderIds={pending}
      />
    </ScrollArea>
  );
}

function RefreshButton({ refreshing, onRefresh }: { refreshing: boolean; onRefresh: () => void }) {
  return (
    <Button
      variant="quiet"
      size="icon-sm"
      aria-label={refreshing ? "Refreshing provider usage" : "Refresh provider usage"}
      aria-busy={refreshing || undefined}
      disabled={refreshing}
      onClick={onRefresh}
    >
      {refreshing ? (
        <Spinner aria-hidden="true" className="size-3.5" />
      ) : (
        <Icon as={RefreshCw} className="size-3.5" />
      )}
    </Button>
  );
}

function PanelFooter({
  onOpenUsage,
  onOpenSettings,
}: Pick<ProviderUsagePanelProps, "onOpenUsage" | "onOpenSettings">) {
  if (!onOpenUsage && !onOpenSettings) return null;
  return (
    <footer className="flex items-center justify-between gap-2 border-t border-line-soft px-1 pt-2">
      {onOpenUsage ? (
        <Button variant="quiet" size="sm" onClick={onOpenUsage}>
          Open Usage
        </Button>
      ) : (
        <span />
      )}
      {onOpenSettings ? (
        <Button variant="quiet" size="sm" onClick={onOpenSettings}>
          Settings
        </Button>
      ) : null}
    </footer>
  );
}

/**
 * Header (title, freshness, Refresh), one collapsed row per target in the
 * daemon's order, and the footer links. Serves the tray popover and, with
 * `chrome="window"`, the Usage app.
 */
export function ProviderUsagePanel(props: ProviderUsagePanelProps) {
  const {
    viewModel,
    daemonUnavailable = false,
    chrome = "popover",
    title = PROVIDER_USAGE_TITLE,
    onRefresh,
    refreshing = false,
    className,
  } = props;
  const now = useUsageClock();
  const freshness = viewModel?.enabled ? formatUpdated(viewModel.oldestMeasuredAt, now) : null;
  // Refresh is absent, not disabled, while the daemon is unreachable.
  const canRefresh = !daemonUnavailable && viewModel?.enabled !== false;

  return (
    <section
      data-slot="provider-usage-panel"
      data-chrome={chrome}
      aria-label={title}
      className={cn("flex min-h-0 min-w-0 flex-col gap-2", className)}
    >
      <header className="flex items-start gap-2 px-2 pt-1">
        <PopoverHeader className="min-w-0 flex-1 justify-center self-stretch">
          {chrome === "popover" ? <PopoverTitle>{title}</PopoverTitle> : null}
          {freshness ? (
            <p data-slot="provider-usage-freshness" className="text-meta text-muted">
              {freshness}
            </p>
          ) : null}
        </PopoverHeader>
        {onRefresh && canRefresh ? (
          <RefreshButton refreshing={refreshing} onRefresh={onRefresh} />
        ) : null}
      </header>
      {daemonUnavailable ? (
        <Alert variant="warning" className="mx-2 w-auto">
          <Icon as={Unplug} />
          <AlertTitle>Daemon unavailable</AlertTitle>
          <AlertDescription>
            {viewModel ? "Showing the last usage this window received." : "Usage can't load."}
          </AlertDescription>
        </Alert>
      ) : null}
      <PanelBody props={props} now={now} />
      <PanelFooter onOpenUsage={props.onOpenUsage} onOpenSettings={props.onOpenSettings} />
    </section>
  );
}
