import { useState } from "react";
import {
  ChevronRight,
  Ellipsis,
  Pencil,
  RefreshCw,
  SlidersHorizontal,
  Sparkles,
  Trash2,
  TriangleAlert,
  Wrench,
} from "@compozy/ui/icons";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  ListingRow,
  Pill,
  StateGlyph,
  Switch,
} from "@compozy/ui";

import {
  DISPLAYED_REMOTE_SCOPES,
  type ComputerFixture,
  type ComputerRouteFixture,
} from "../mocks/remote-computers-fixtures";
import { ComputerLoadMeter } from "./computer-load-meter";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { ComputerStatusChip } from "./computer-status-chip";

type ExpandedDisclosure = "routes" | "permissions" | null;

const ROUTE_KIND_LABEL: Record<ComputerRouteFixture["kind"], string> = {
  direct: "LAN",
  ssh: "SSH",
  cloud: "Hosted route",
};

export interface ComputerRowProps {
  computer: ComputerFixture;
  selected?: boolean;
  /** Initial open disclosure for prototype showcase or restored state. */
  defaultDisclosure?: ExpandedDisclosure;
  onToggleEnabled?: (computer: ComputerFixture, nextEnabled: boolean) => void;
  onOpenDetails?: (
    computer: ComputerFixture,
    initialTab?: "status" | "providers" | "update"
  ) => void;
  onRename?: (computer: ComputerFixture) => void;
  onRemove?: (computer: ComputerFixture) => void;
  onRemoveRoute?: (computer: ComputerFixture, routeId: string) => void;
  onUpgrade?: () => void;
}

function primaryRouteText(computer: ComputerFixture): string | null {
  if (computer.routeInUse) return computer.routeInUse.label;
  if (computer.status === "connecting" && computer.routes[0]) {
    return computer.routes[0].label;
  }
  if (computer.status === "incompatible" && computer.sshTarget) {
    return `via SSH ${computer.sshTarget}`;
  }
  if (computer.status === "disabled" && computer.routes[0]) {
    return computer.routes[0].label;
  }
  return null;
}

function ComputerRoutesPanel({
  computer,
  onRemoveRoute,
}: {
  computer: ComputerFixture;
  onRemoveRoute?: (computer: ComputerFixture, routeId: string) => void;
}) {
  return (
    <div
      className="border-t border-line-soft bg-sunken/70 px-4 py-3 pl-16"
      data-slot="computer-routes-disclosure"
    >
      <div className="mb-2 flex items-center justify-between gap-2">
        <span className="eyebrow text-subtle">
          Routes in priority order — switches automatically
        </span>
      </div>
      <ol className="flex flex-col divide-y divide-line-soft">
        {computer.routes.map((route, idx) => {
          const canRemove = route.learned && computer.routes.length > 1;
          return (
            <li
              className="flex min-h-9 flex-wrap items-center justify-between gap-2 py-1.5 first:pt-0 last:pb-0"
              key={route.id}
            >
              <div className="flex min-w-0 flex-wrap items-center gap-2">
                <span className="w-4 text-eyebrow text-faint tabular-nums">{idx + 1}.</span>
                <span className="text-small-body font-medium text-fg">
                  {ROUTE_KIND_LABEL[route.kind]}
                </span>
                <span className="truncate font-mono text-mono-id text-muted-foreground">
                  {route.address}
                </span>
                {route.inUse ? (
                  <Pill size="xs" tone="success">
                    In use
                  </Pill>
                ) : null}
                {route.learned ? (
                  <Pill size="xs" tone="neutral">
                    Learned
                  </Pill>
                ) : null}
                {route.identityMismatch ? (
                  <Pill size="xs" tone="warning">
                    Different computer at this address
                  </Pill>
                ) : null}
              </div>
              {canRemove && onRemoveRoute ? (
                <Button
                  onClick={() => onRemoveRoute(computer, route.id)}
                  size="xs"
                  type="button"
                  variant="ghost"
                >
                  Remove
                </Button>
              ) : null}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function ComputerPermissionsPanel({ computer }: { computer: ComputerFixture }) {
  const granted = new Set(computer.scopes);
  return (
    <div
      className="border-t border-line-soft bg-sunken/70 px-4 py-3 pl-16"
      data-slot="computer-permissions-disclosure"
    >
      <div className="grid grid-cols-2 gap-x-6 gap-y-2 sm:grid-cols-3">
        {DISPLAYED_REMOTE_SCOPES.map(item => {
          const allowed = granted.has(item.scope);
          return (
            <div className="flex items-center gap-2 text-meta" key={item.scope}>
              <StateGlyph size="sm" state={allowed ? "done" : "stopped"} />
              <span className={allowed ? "text-fg" : "text-faint"}>{item.label}</span>
              <span className="text-badge text-faint">{allowed ? "Allowed" : "Not granted"}</span>
            </div>
          );
        })}
      </div>
      <p className="mt-2.5 text-badge text-faint">
        These permissions apply to this device on {computer.name}. Controls for missing permissions
        stay hidden in remote sessions.
      </p>
    </div>
  );
}

function InlineStatusNotice({
  computer,
  onOpenDetails,
}: {
  computer: ComputerFixture;
  onOpenDetails?: ComputerRowProps["onOpenDetails"];
}) {
  if (computer.status === "offline" && computer.diagnosis) {
    return (
      <div className="mt-1.5 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-meta text-muted-foreground">
        <span>{computer.diagnosis.message}</span>
        {computer.diagnosis.retryInSeconds ? (
          <>
            <ListingRow.MetaDot />
            <span className="tabular-nums">
              retrying in {computer.diagnosis.retryInSeconds}s
              {computer.diagnosis.attempt ? ` (attempt ${computer.diagnosis.attempt})` : ""}
            </span>
          </>
        ) : null}
        <ListingRow.MetaDot />
        <button
          className="cursor-pointer font-medium text-fg underline-offset-3 hover:underline focus-visible:shadow-focus-ring focus-visible:outline-none"
          onClick={() => onOpenDetails?.(computer, "status")}
          type="button"
        >
          Details ›
        </button>
      </div>
    );
  }

  if (computer.status === "needs_attention" && computer.attentionReason) {
    return (
      <div className="mt-1.5 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-meta text-warning">
        <span>{computer.attentionReason.summary}</span>
        <ListingRow.MetaDot />
        <button
          className="cursor-pointer font-medium text-fg underline-offset-3 hover:underline focus-visible:shadow-focus-ring focus-visible:outline-none"
          onClick={() =>
            onOpenDetails?.(
              computer,
              computer.attentionReason?.kind === "provider_signed_out" ? "providers" : "status"
            )
          }
          type="button"
        >
          {computer.attentionReason.actionLabel} ›
        </button>
      </div>
    );
  }

  if (computer.status === "incompatible") {
    return (
      <div className="mt-1.5 flex flex-wrap items-center gap-x-1.5 gap-y-0.5 text-meta text-warning">
        <span>
          {computer.name} runs Compozy {computer.version}. Update it to connect.
        </span>
        <ListingRow.MetaDot />
        <button
          className="cursor-pointer font-medium text-fg underline-offset-3 hover:underline focus-visible:shadow-focus-ring focus-visible:outline-none"
          onClick={() => onOpenDetails?.(computer, "update")}
          type="button"
        >
          Details ›
        </button>
      </div>
    );
  }

  return null;
}

/**
 * Roomy, flat-depth computer row (`_uiux.md` U1, `research-references` §3.1):
 * - Identity-only dimming when turned off so Switch and `⋯` remain sharp
 * - Contextual `Upgrade` / `Update` 1-click CTA right before the Switch
 * - Inline hairline disclosures for `N routes ›` and `Permissions ›`
 */
export function ComputerRow({
  computer,
  selected = false,
  defaultDisclosure = null,
  onToggleEnabled,
  onOpenDetails,
  onRename,
  onRemove,
  onRemoveRoute,
  onUpgrade,
}: ComputerRowProps) {
  const [openDisclosure, setOpenDisclosure] = useState<ExpandedDisclosure>(defaultDisclosure);
  const dimmed = !computer.enabled || computer.status === "needs_pro";
  const routeText = primaryRouteText(computer);
  const showLoad = computer.status === "connected" || computer.status === "needs_attention";
  const showAttentionIcon =
    computer.status === "needs_attention" || computer.status === "incompatible";
  const routesLabel = computer.routes.length === 1 ? "1 route" : `${computer.routes.length} routes`;

  const toggleDisclosure = (next: Exclude<ExpandedDisclosure, null>) => {
    setOpenDisclosure(current => (current === next ? null : next));
  };

  return (
    <div
      className="border-b border-line-soft last:border-b-0"
      data-computer-id={computer.id}
      data-computer-status={computer.status}
      data-slot="computer-row-wrap"
    >
      <ListingRow className="min-h-15 border-b-0 py-3.5" interactive={false} selected={selected}>
        <ListingRow.Icon className={cn(dimmed && "opacity-60")}>
          <ComputerPlatformIcon
            iconKind={computer.iconKind}
            platform={computer.platform}
            size="sm"
            tone="muted"
          />
        </ListingRow.Icon>

        <ListingRow.Main className={cn(dimmed && "opacity-60")}>
          <ListingRow.Name>
            <ListingRow.Title>{computer.name}</ListingRow.Title>
            {showAttentionIcon ? (
              <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
            ) : null}
            <ComputerStatusChip
              onInspect={onOpenDetails ? () => onOpenDetails(computer, "status") : undefined}
              status={computer.status}
            />
          </ListingRow.Name>

          <ListingRow.Meta className="mt-1">
            {routeText ? <span className="truncate text-muted-foreground">{routeText}</span> : null}
            {showLoad ? (
              <>
                {routeText ? <ListingRow.MetaDot /> : null}
                <ComputerLoadMeter computerName={computer.name} load={computer.load} />
              </>
            ) : null}
            {computer.routes.length > 0 ? (
              <>
                {routeText || showLoad ? <ListingRow.MetaDot /> : null}
                <button
                  aria-expanded={openDisclosure === "routes"}
                  className={cn(
                    "inline-flex cursor-pointer items-center gap-0.5 rounded-xs text-eyebrow transition-colors duration-base hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
                    openDisclosure === "routes" ? "font-medium text-fg" : "text-muted-foreground"
                  )}
                  onClick={() => toggleDisclosure("routes")}
                  type="button"
                >
                  <span>{routesLabel}</span>
                  <ChevronRight
                    aria-hidden="true"
                    className={cn(
                      "size-3 transition-transform duration-base",
                      openDisclosure === "routes" && "rotate-90"
                    )}
                  />
                </button>
              </>
            ) : null}
            <ListingRow.MetaDot />
            <button
              aria-expanded={openDisclosure === "permissions"}
              className={cn(
                "inline-flex cursor-pointer items-center gap-0.5 rounded-xs text-eyebrow transition-colors duration-base hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
                openDisclosure === "permissions" ? "font-medium text-fg" : "text-muted-foreground"
              )}
              onClick={() => toggleDisclosure("permissions")}
              type="button"
            >
              <span>Permissions</span>
              <ChevronRight
                aria-hidden="true"
                className={cn(
                  "size-3 transition-transform duration-base",
                  openDisclosure === "permissions" && "rotate-90"
                )}
              />
            </button>
          </ListingRow.Meta>

          <InlineStatusNotice computer={computer} onOpenDetails={onOpenDetails} />
        </ListingRow.Main>

        <ListingRow.Trail className="gap-2.5">
          {computer.status === "needs_pro" ? (
            <Button onClick={onUpgrade} size="xs" type="button" variant="secondary">
              <Sparkles aria-hidden="true" />
              Upgrade
            </Button>
          ) : null}
          {computer.status === "incompatible" && computer.originKind === "ssh" ? (
            <Button
              onClick={() => onOpenDetails?.(computer, "update")}
              size="xs"
              type="button"
              variant="secondary"
            >
              <RefreshCw aria-hidden="true" />
              Update
            </Button>
          ) : null}
          <Switch
            aria-label={`Enable ${computer.name}`}
            checked={computer.enabled && computer.status !== "needs_pro"}
            disabled={computer.status === "needs_pro"}
            onCheckedChange={next => onToggleEnabled?.(computer, next)}
            size="sm"
          />
          <DropdownMenu>
            <DropdownMenuTrigger
              render={
                <Button
                  aria-label={`Actions for ${computer.name}`}
                  size="icon-sm"
                  type="button"
                  variant="ghost"
                />
              }
            >
              <Ellipsis aria-hidden="true" className="size-4" />
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="min-w-44">
              <DropdownMenuItem onClick={() => onOpenDetails?.(computer, "status")}>
                <SlidersHorizontal aria-hidden="true" />
                Details
              </DropdownMenuItem>
              <DropdownMenuItem onClick={() => onOpenDetails?.(computer, "providers")}>
                <Wrench aria-hidden="true" />
                Providers
              </DropdownMenuItem>
              {computer.status === "incompatible" ? (
                <DropdownMenuItem onClick={() => onOpenDetails?.(computer, "update")}>
                  <RefreshCw aria-hidden="true" />
                  Update
                </DropdownMenuItem>
              ) : null}
              <DropdownMenuItem onClick={() => onRename?.(computer)}>
                <Pencil aria-hidden="true" />
                Rename
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              <DropdownMenuItem onClick={() => onRemove?.(computer)} variant="destructive">
                <Trash2 aria-hidden="true" />
                Remove
              </DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </ListingRow.Trail>
      </ListingRow>

      {openDisclosure === "routes" ? (
        <ComputerRoutesPanel computer={computer} onRemoveRoute={onRemoveRoute} />
      ) : null}
      {openDisclosure === "permissions" ? <ComputerPermissionsPanel computer={computer} /> : null}
    </div>
  );
}
