import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Logo,
  Time,
} from "@agh/ui";

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
            <DialogTitle>AGH</DialogTitle>
            <DialogDescription>The agent operating system running this desktop.</DialogDescription>
          </div>
        </DialogHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-5 py-4">
          {daemon.isPending ? (
            <p className="text-small-body text-muted" role="status">
              Reading daemon status…
            </p>
          ) : null}
          {daemon.isError ? (
            <p className="text-small-body text-danger" role="status">
              Daemon status is unavailable. The desktop keeps running on its last snapshot.
            </p>
          ) : null}
          {rows.length > 0 ? (
            <dl className="flex flex-col">
              {rows.map(row => (
                <div
                  key={row.id}
                  data-testid={`os-about-row-${row.id}`}
                  className="flex min-h-7 items-start justify-between gap-6 border-b border-line-soft py-1.5 last:border-b-0"
                >
                  <dt className="shrink-0 text-small-body text-muted">{row.label}</dt>
                  <dd className="min-w-0 truncate text-right font-mono text-micro text-fg">
                    {row.value}
                  </dd>
                </div>
              ))}
            </dl>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  );
}
