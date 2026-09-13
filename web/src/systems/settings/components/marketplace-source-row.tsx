import { Link } from "@tanstack/react-router";
import { ChevronDown, RefreshCw } from "lucide-react";

import {
  Button,
  cn,
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
  Eyebrow,
  MonoId,
  Pill,
  PropertyRow,
  Spinner,
  Switch,
  Time,
} from "@compozy/ui";

import type { MarketplaceSource } from "@/systems/marketplace";

import {
  marketplaceSourceCountLabel,
  marketplaceSourceDegraded,
  marketplaceSourceDegradedSentence,
  marketplaceSourceOrigin,
  marketplaceSourcePluginsLine,
  marketplaceSourceReason,
  marketplaceSourceTestId,
  marketplaceSourceWasRead,
} from "./marketplace-source-labels";

export interface MarketplaceSourceRowProps {
  source: MarketplaceSource;
  /** A mutation for this row is in flight; its controls wait. */
  pending: boolean;
  /** Absent for the always-on feed. */
  onToggle?: (enabled: boolean) => void;
  onRefresh: () => void;
  /** Present for custom sources only — presets are turned off, never removed. */
  onRemove?: () => void;
}

/**
 * One source, one 44px line: chevron · name · custom pill · degraded pill · count · switch. The
 * origin, document, last read, counts, reason, and diagnostics live behind the disclosure so a
 * dozen marketplaces still read as a list. A degraded source stays visible on the closed line.
 */
export function MarketplaceSourceRow({
  source,
  pending,
  onToggle,
  onRefresh,
  onRemove,
}: MarketplaceSourceRowProps) {
  const testId = marketplaceSourceTestId(source.name);
  const origin = marketplaceSourceOrigin(source);
  const degraded = marketplaceSourceDegraded(source);
  const wasRead = marketplaceSourceWasRead(source);
  const reason = marketplaceSourceReason(source);
  const alwaysOn = source.kind === "feed";

  return (
    <Collapsible
      className="flex min-w-0 flex-col border-b border-line last:border-b-0"
      data-state-word={source.state}
      data-testid={testId}
    >
      <div className="flex min-h-11 min-w-0 items-center gap-2 px-3">
        <CollapsibleTrigger
          className={cn(
            "group/marketplace-source flex min-w-0 flex-1 items-center gap-2 py-2 text-left",
            "rounded-sm focus-visible:shadow-focus-ring focus-visible:outline-none"
          )}
          data-testid={`${testId}-disclosure`}
          type="button"
        >
          <ChevronDown
            aria-hidden="true"
            className={cn(
              "size-3.5 shrink-0 -rotate-90 text-faint",
              "transition-transform duration-base group-data-panel-open/marketplace-source:rotate-0"
            )}
          />
          <span className="truncate text-sm text-fg">{source.name}</span>
          {source.kind === "custom" ? (
            <Pill data-testid={`${testId}-custom`} form="hollow" size="xs">
              custom
            </Pill>
          ) : null}
          {degraded ? (
            <Pill data-testid={`${testId}-degraded`} size="xs" tone="warning">
              couldn&rsquo;t refresh
            </Pill>
          ) : null}
        </CollapsibleTrigger>
        <div className="flex shrink-0 items-center gap-2">
          {pending ? <Spinner aria-hidden="true" className="size-3 text-subtle" /> : null}
          {!source.enabled ? (
            <span className="text-form-hint text-subtle" data-testid={`${testId}-off`}>
              off
            </span>
          ) : wasRead ? (
            <Pill data-testid={`${testId}-count`} size="xs">
              {marketplaceSourceCountLabel(source)}
            </Pill>
          ) : null}
          {alwaysOn ? (
            <Pill data-testid={`${testId}-always-on`} size="xs">
              always on
            </Pill>
          ) : null}
          {onToggle ? (
            <Switch
              aria-label={`List ${source.name} in the Marketplace`}
              checked={source.enabled}
              data-testid={`${testId}-toggle`}
              disabled={pending}
              onCheckedChange={onToggle}
              size="sm"
            />
          ) : null}
        </div>
      </div>
      <CollapsibleContent>
        <div className="flex min-w-0 flex-col gap-1 px-3 pb-2.5 pl-8.5">
          {degraded ? (
            <p className="text-small-body text-fg" data-testid={`${testId}-sentence`}>
              {marketplaceSourceDegradedSentence(source, origin)}
              {wasRead && source.plugins > 0 ? (
                <>
                  {" "}
                  Its {marketplaceSourceCountLabel(source)} still show in the Marketplace from the
                  last read, {source.last_read_at ? <Time iso={source.last_read_at} /> : null}.
                </>
              ) : null}
            </p>
          ) : null}
          {!source.enabled && !wasRead ? (
            <p className="text-small-body text-muted" data-testid={`${testId}-sentence`}>
              Turn it on to read its plugin list. Compozy will not contact this{" "}
              {origin.kind === "feed" ? "feed" : origin.kind} until then.
            </p>
          ) : null}
          <PropertyRow
            data-testid={`${testId}-origin`}
            label={origin.label}
            mono
            valueTitle={origin.value}
          >
            {origin.value}
          </PropertyRow>
          {source.document_path ? (
            <PropertyRow data-testid={`${testId}-document`} label="Document" mono>
              {source.owner
                ? `${source.document_path} · owner ${source.owner}`
                : source.document_path}
            </PropertyRow>
          ) : null}
          <PropertyRow data-testid={`${testId}-last-read`} label="Last read">
            {source.last_read_at ? <Time iso={source.last_read_at} /> : "never"}
          </PropertyRow>
          {wasRead ? (
            <PropertyRow data-testid={`${testId}-plugins`} label="Plugins">
              {marketplaceSourcePluginsLine(source)}
            </PropertyRow>
          ) : null}
          {degraded && reason ? (
            <PropertyRow data-testid={`${testId}-reason`} label="Reason" mono>
              {reason}
            </PropertyRow>
          ) : null}
          {source.diagnostics.length > 0 ? (
            <MarketplaceSourceDiagnostics diagnostics={source.diagnostics} testId={testId} />
          ) : null}
          <div className="flex flex-wrap items-center gap-1 pt-1.5">
            {source.enabled ? (
              <Button
                data-testid={`${testId}-refresh`}
                disabled={pending}
                onClick={onRefresh}
                size="sm"
                type="button"
                variant="ghost"
              >
                <RefreshCw aria-hidden="true" className="size-3" />
                {degraded ? "Try again" : "Refresh now"}
              </Button>
            ) : null}
            {source.enabled && wasRead && source.plugins > 0 ? (
              <Button
                data-testid={`${testId}-show`}
                nativeButton={false}
                render={<Link to="/marketplace" />}
                size="sm"
                variant="ghost"
              >
                Show in Marketplace
              </Button>
            ) : null}
            {onRemove ? (
              <Button
                className="ml-auto text-danger hover:text-danger"
                data-testid={`${testId}-remove`}
                disabled={pending}
                onClick={onRemove}
                size="sm"
                type="button"
                variant="ghost"
              >
                Remove
              </Button>
            ) : null}
          </div>
        </div>
      </CollapsibleContent>
    </Collapsible>
  );
}

/** Daemon diagnostics under the reason: one reported message per line, its code in micro mono. */
function MarketplaceSourceDiagnostics({
  diagnostics,
  testId,
}: {
  diagnostics: MarketplaceSource["diagnostics"];
  testId: string;
}) {
  return (
    <div className="flex min-w-0 flex-col gap-1 py-1" data-testid={`${testId}-diagnostics`}>
      <Eyebrow>Diagnostics</Eyebrow>
      <ul className="flex min-w-0 flex-col gap-0.5">
        {diagnostics.map(diagnostic => (
          <li
            className="flex min-w-0 flex-wrap items-baseline gap-x-2 text-form-hint text-muted"
            key={diagnostic.id || `${diagnostic.code}:${diagnostic.message}`}
          >
            <MonoId className="shrink-0 text-faint" preserveCase value={diagnostic.code} />
            <span className="min-w-0">{diagnostic.message}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}
