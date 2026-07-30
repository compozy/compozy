import { cn, CopyIconButton } from "@compozy/ui";

export function MarketplaceInstallCommand({
  command,
  className,
}: {
  command: string;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "flex min-w-0 items-center gap-2.5 rounded-lg border border-line bg-canvas py-2 ps-3.5 pe-1.5",
        className
      )}
    >
      <span aria-hidden className="font-mono text-small-body text-subtle select-none">
        $
      </span>
      <code className="min-w-0 flex-1 truncate font-mono text-small-body text-fg">{command}</code>
      <CopyIconButton
        value={command}
        aria-label={`Copy ${command}`}
        className="shrink-0 text-subtle"
      />
    </div>
  );
}
