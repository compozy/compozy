import { ChevronRight } from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Logo,
  MetadataList,
  MetadataListRow,
  Time,
} from "@compozy/ui";

import { useDaemonStatus, useStatus } from "@/systems/status";

export interface OsAboutDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const UNKNOWN = "—";

/**
 * Installation identity, straight from `/api/status`. Only fields the daemon
 * actually puts on the wire are shown — build SHA, Go version, and OS/arch live
 * in `internal/version` but are never published, so they are not invented here.
 */
export function OsAboutDialog({ open, onOpenChange }: OsAboutDialogProps) {
  const daemon = useDaemonStatus();
  const status = useStatus();
  const rows = daemon.data
    ? [
        { id: "version", label: "Version", value: daemon.data.version ?? UNKNOWN },
        { id: "status", label: "Status", value: daemon.data.status },
        {
          id: "started",
          label: "Started",
          value: <Time iso={daemon.data.started_at} />,
        },
      ]
    : [];
  // Operator detail stays one step deeper: closed by default (calm defaults).
  const technicalRows = daemon.data
    ? [
        { id: "pid", label: "Process", value: String(daemon.data.pid) },
        { id: "http", label: "HTTP", value: `${daemon.data.http_host}:${daemon.data.http_port}` },
        { id: "socket", label: "Socket", value: daemon.data.socket },
        { id: "home", label: "Home directory", value: daemon.data.user_home_dir },
        {
          id: "config",
          label: "Config file",
          value: status.data?.config.config_file ?? UNKNOWN,
        },
      ]
    : [];

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        unframed
        data-testid="os-about-dialog"
        className="flex max-h-[min(var(--height-modal-md),80vh)] w-full flex-col sm:max-w-(--width-modal-sm)"
      >
        <DialogHeader variant="ruled" className="flex-row items-center gap-3">
          <Logo variant="symbol" decorative className="size-6" />
          <div className="flex min-w-0 flex-col gap-1">
            <DialogTitle>CompozyOS</DialogTitle>
            <DialogDescription>An operating system for AI agents.</DialogDescription>
          </div>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {daemon.isPending ? (
            <p className="text-small-body text-muted" role="status">
              Reading CompozyOS status…
            </p>
          ) : null}
          {daemon.isError ? (
            <p className="text-small-body text-danger" role="status">
              CompozyOS status is unavailable. The desktop keeps running on its last snapshot.
            </p>
          ) : null}
          {rows.length > 0 ? (
            <div className="flex flex-col gap-4">
              <AboutRows rows={rows} />
              <Collapsible>
                <CollapsibleTrigger className="group/about-details flex items-center gap-1 text-small-body text-muted hover:text-fg-strong">
                  <ChevronRight
                    aria-hidden="true"
                    className="size-3.5 transition-transform duration-base group-data-[panel-open]/about-details:rotate-90"
                  />
                  Technical details
                </CollapsibleTrigger>
                <CollapsibleContent className="pt-2">
                  <AboutRows rows={technicalRows} />
                </CollapsibleContent>
              </Collapsible>
            </div>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function AboutRows({
  rows,
}: {
  rows: readonly { id: string; label: string; value: React.ReactNode }[];
}) {
  return (
    <MetadataList>
      {rows.map(row => (
        <MetadataListRow
          key={row.id}
          data-testid={`os-about-row-${row.id}`}
          label={row.label}
          valueProps={{ className: "truncate font-mono text-micro text-fg" }}
        >
          {row.value}
        </MetadataListRow>
      ))}
    </MetadataList>
  );
}
