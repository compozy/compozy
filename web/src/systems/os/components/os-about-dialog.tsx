import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  Disclosure,
  Logo,
  MetadataList,
  MetadataListRow,
  Skeleton,
  SkeletonRegion,
  SkeletonText,
  Time,
} from "@compozy/ui";

import { useDaemonStatus, useStatus } from "@/systems/status";

export interface OsAboutDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

const UNKNOWN = "—";

/** The headline rows' labels: static copy, so the loading shape names them too. */
const ABOUT_LABELS = { version: "Version", status: "Status", started: "Started" } as const;
const ABOUT_VALUE_PROPS = { className: "truncate font-mono text-micro text-fg" };

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
        { id: "version", label: ABOUT_LABELS.version, value: daemon.data.version ?? UNKNOWN },
        { id: "status", label: ABOUT_LABELS.status, value: daemon.data.status },
        {
          id: "started",
          label: ABOUT_LABELS.started,
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
          {daemon.isPending ? <AboutRowsSkeleton /> : null}
          {daemon.isError ? (
            <p className="text-small-body text-destructive" role="status">
              CompozyOS status is unavailable. The desktop keeps running on its last snapshot.
            </p>
          ) : null}
          {rows.length > 0 ? (
            <div className="flex flex-col gap-4">
              <AboutRows rows={rows} />
              <Disclosure label="Technical details" size="md">
                <AboutRows rows={technicalRows} />
              </Disclosure>
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
          valueProps={ABOUT_VALUE_PROPS}
        >
          {row.value}
        </MetadataListRow>
      ))}
    </MetadataList>
  );
}

/**
 * The identity rows while the daemon status is read: the real labels over blocks for the values,
 * and the closed Technical details fold as a chevron and a line, under one busy region.
 */
function AboutRowsSkeleton() {
  return (
    <SkeletonRegion className="flex flex-col gap-4" label="Reading CompozyOS status">
      <MetadataList aria-hidden="true">
        {Object.values(ABOUT_LABELS).map(label => (
          <MetadataListRow key={label} label={label} valueProps={ABOUT_VALUE_PROPS}>
            <SkeletonText className="w-32" />
          </MetadataListRow>
        ))}
      </MetadataList>
      <div aria-hidden="true" className="flex w-fit items-center gap-2 py-1 text-small-body">
        <Skeleton className="size-3.5 shrink-0 rounded-xs" />
        <SkeletonText className="w-28" />
      </div>
    </SkeletonRegion>
  );
}
