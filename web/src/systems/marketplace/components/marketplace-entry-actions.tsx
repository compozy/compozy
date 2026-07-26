import {
  Button,
  Pill,
  Spinner,
  Tooltip,
  TooltipContent,
  TooltipTrigger,
  buttonVariants,
} from "@agh/ui";

import type { MarketplaceListing } from "../types";

function MarketplaceEntryStatus({ entry }: { entry: MarketplaceListing }) {
  if (entry.update_available) {
    return (
      <Pill mono tone="warning">
        {entry.version ? `v${entry.version} available` : "update available"}
      </Pill>
    );
  }
  if (entry.installed) {
    return (
      <Pill mono tone="success">
        {entry.kind === "bundle" ? "active" : "installed"}
      </Pill>
    );
  }
  if (entry.kind === "extension" && entry.trust) {
    const warningCount = entry.trust.warnings?.length ?? 0;
    if (entry.trust.decision === "verified") {
      return (
        <Pill mono tone="success">
          verified
        </Pill>
      );
    }
    return (
      <Pill mono tone={entry.trust.decision === "blocked" ? "danger" : "warning"}>
        {entry.trust.decision === "blocked" ? "blocked" : "unverified"}
        {warningCount > 0 ? ` · ${warningCount}` : ""}
      </Pill>
    );
  }
  if (entry.kind === "mcp") {
    return (
      <Pill mono tone="info">
        curated
      </Pill>
    );
  }
  return null;
}

function MarketplaceEntryAction({
  entry,
  emphasis = "neutral",
  pending = false,
  onAction,
}: {
  entry: MarketplaceListing;
  emphasis?: "neutral" | "primary";
  pending?: boolean;
  onAction: (entry: MarketplaceListing) => void;
}) {
  const blocked = entry.kind === "extension" && entry.trust?.decision === "blocked";
  const action = marketplaceEntryActionLabel(entry);
  if (entry.installed && !entry.update_available) {
    if (!entry.manage_path) return null;
    return (
      <a
        aria-label={`Manage ${entry.name}`}
        className={buttonVariants({
          size: "sm",
          variant: emphasis === "primary" ? "default" : "ghost",
        })}
        href={entry.manage_path}
      >
        Manage
      </a>
    );
  }

  if (blocked) {
    return (
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-disabled="true"
              aria-label={`Install ${entry.name}, blocked by extensions policy`}
              data-testid={`marketplace-action-${entry.entry_id}`}
              onClick={event => event.preventDefault()}
              size="sm"
              type="button"
              variant="neutral"
            />
          }
        >
          Install
        </TooltipTrigger>
        <TooltipContent>Blocked by extensions policy</TooltipContent>
      </Tooltip>
    );
  }

  return (
    <Button
      aria-label={`${action} ${entry.name}`}
      data-testid={`marketplace-action-${entry.entry_id}`}
      disabled={pending}
      onClick={() => onAction(entry)}
      size="sm"
      type="button"
      variant={emphasis === "primary" ? "default" : "neutral"}
    >
      {pending ? <Spinner aria-hidden="true" className="size-3" /> : null}
      {pending ? marketplaceEntryPendingLabel(entry) : action}
    </Button>
  );
}

function marketplaceEntryActionLabel(entry: MarketplaceListing): string {
  if (entry.update_available) return "Update";
  if (entry.kind === "bundle") return "Activate";
  return "Install";
}

function marketplaceEntryPendingLabel(entry: MarketplaceListing): string {
  if (entry.update_available) return "Updating…";
  if (entry.kind === "bundle") return "Activating…";
  return "Installing…";
}

export { MarketplaceEntryAction, MarketplaceEntryStatus };
