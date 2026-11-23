import { ArrowDown, ArrowUp, Trash2, TriangleAlert } from "@compozy/ui/icons";

import { Button, MonoId, Pill, StateGlyph, Switch, toast } from "@compozy/ui";

import {
  DISPLAYED_REMOTE_SCOPES,
  type ComputerRouteFixture,
  type RemoteScope,
} from "../mocks/remote-computers-fixtures";

const ROUTE_KIND_LABEL: Record<ComputerRouteFixture["kind"], string> = {
  direct: "Direct LAN",
  ssh: "SSH",
  cloud: "Hosted route",
};

export interface ComputerDetailsRoutesTabProps {
  computerName: string;
  routes: readonly ComputerRouteFixture[];
  onMoveRoute?: (routeId: string, direction: "up" | "down") => void;
  onRemoveRoute?: (routeId: string) => void;
}

/** Ordered priority list of routes (`LAN` > `SSH` > `Hosted route`) with `In use`, `Learned`, and mismatch warnings. */
export function ComputerDetailsRoutesTab({
  computerName,
  routes,
  onMoveRoute,
  onRemoveRoute,
}: ComputerDetailsRoutesTabProps) {
  return (
    <div className="flex flex-col gap-4" data-slot="computer-details-routes-tab">
      <p className="text-small-body text-muted-foreground">
        CompozyOS tries routes in priority order and switches to LAN automatically when{" "}
        <span className="font-medium text-fg">{computerName}</span> is on the same network.
      </p>

      <ol className="flex flex-col divide-y divide-line-soft border-t border-line-soft">
        {routes.map((route, idx) => {
          const canRemove = route.learned && routes.length > 1;
          return (
            <li className="flex flex-col gap-1.5 py-3" data-route-id={route.id} key={route.id}>
              <div className="flex items-start justify-between gap-2">
                <div className="flex min-w-0 flex-1 items-start gap-2.5">
                  <span className="mt-0.5 w-4 shrink-0 text-eyebrow text-faint tabular-nums">
                    {idx + 1}.
                  </span>
                  <div className="flex min-w-0 flex-1 flex-col gap-1">
                    <div className="flex flex-wrap items-center gap-1.5">
                      <span className="text-small-body font-medium text-fg-strong">
                        {ROUTE_KIND_LABEL[route.kind]}
                      </span>
                      {route.inUse ? (
                        <Pill size="xs" tone="success">
                          In use
                        </Pill>
                      ) : null}
                      {route.learned ? (
                        <Pill size="xs" tone="neutral">
                          Found automatically
                        </Pill>
                      ) : null}
                    </div>
                    <MonoId
                      className="max-w-full text-muted-foreground"
                      copy
                      preserveCase
                      value={route.address}
                    />
                  </div>
                </div>

                <div className="flex shrink-0 items-center gap-0.5">
                  <Button
                    aria-label={`Move ${ROUTE_KIND_LABEL[route.kind]} route up`}
                    disabled={idx === 0}
                    onClick={() => onMoveRoute?.(route.id, "up")}
                    size="icon-xs"
                    type="button"
                    variant="ghost"
                  >
                    <ArrowUp aria-hidden="true" className="size-3.5" />
                  </Button>
                  <Button
                    aria-label={`Move ${ROUTE_KIND_LABEL[route.kind]} route down`}
                    disabled={idx === routes.length - 1}
                    onClick={() => onMoveRoute?.(route.id, "down")}
                    size="icon-xs"
                    type="button"
                    variant="ghost"
                  >
                    <ArrowDown aria-hidden="true" className="size-3.5" />
                  </Button>
                  <Button
                    aria-label={`Remove ${ROUTE_KIND_LABEL[route.kind]} route`}
                    disabled={!canRemove}
                    onClick={() => onRemoveRoute?.(route.id)}
                    size="icon-xs"
                    title={
                      canRemove
                        ? "Remove learned route"
                        : route.learned
                          ? "Can't remove the last route"
                          : "Configured routes stay while this computer is added"
                    }
                    type="button"
                    variant="ghost"
                  >
                    <Trash2 aria-hidden="true" className="size-3.5" />
                  </Button>
                </div>
              </div>

              {route.identityMismatch ? (
                <div className="ml-6.5 flex flex-wrap items-center gap-x-2 gap-y-1 text-meta">
                  <span className="inline-flex items-center gap-1.5 font-medium text-warning">
                    <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0" />
                    <span>Different computer at this address</span>
                  </span>
                  <span className="text-muted-foreground">No credentials were sent.</span>
                  {canRemove ? (
                    <button
                      className="cursor-pointer font-medium text-fg underline-offset-3 hover:underline focus-visible:shadow-focus-ring focus-visible:outline-none"
                      onClick={() => onRemoveRoute?.(route.id)}
                      type="button"
                    >
                      Remove
                    </button>
                  ) : null}
                </div>
              ) : route.lastError ? (
                <p className="ml-6.5 text-meta text-muted-foreground">{route.lastError}</p>
              ) : null}
            </li>
          );
        })}
      </ol>
    </div>
  );
}

export interface ComputerDetailsPermissionsTabProps {
  computerName: string;
  scopes: readonly RemoteScope[];
  /** When true, renders interactive switches (admin editing device scopes on this computer). */
  editable?: boolean;
  onToggleScope?: (scope: RemoteScope, next: boolean) => void;
}

/**
 * Effective permissions for this device on `<name>`, supporting both read-only
 * inspection (`_uiux.md` U7) and admin scope editing (`brief-lanes.md` Screen 04).
 */
export function ComputerDetailsPermissionsTab({
  computerName,
  scopes,
  editable = false,
  onToggleScope,
}: ComputerDetailsPermissionsTabProps) {
  const granted = new Set(scopes);
  const allowedCount = DISPLAYED_REMOTE_SCOPES.filter(item => granted.has(item.scope)).length;

  return (
    <div className="flex flex-col gap-4" data-slot="computer-details-permissions-tab">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <p className="text-small-body text-muted-foreground">
          {editable
            ? `Choose what MacBook Pro is allowed to do on ${computerName}.`
            : `Permissions granted to this device on ${computerName}.`}
        </p>
        <Pill size="xs" tone="neutral">
          {allowedCount} of {DISPLAYED_REMOTE_SCOPES.length} allowed
        </Pill>
      </div>

      <div className="overflow-hidden rounded-lg border border-line bg-card">
        {DISPLAYED_REMOTE_SCOPES.map(item => {
          const allowed = granted.has(item.scope);
          return (
            <div
              className="flex min-h-13 items-center justify-between gap-3 border-b border-line-soft px-3.5 py-2.5 last:border-b-0"
              key={item.scope}
            >
              <div className="flex min-w-0 flex-1 items-start gap-2.5">
                {!editable ? (
                  <span className="mt-1 shrink-0">
                    <StateGlyph size="sm" state={allowed ? "done" : "stopped"} />
                  </span>
                ) : null}
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span
                    className={
                      allowed || editable
                        ? "text-small-body font-medium text-fg"
                        : "text-small-body font-medium text-muted-foreground"
                    }
                  >
                    {item.label}
                  </span>
                  <span className="text-meta text-muted-foreground">{item.consequence}</span>
                </div>
              </div>

              {editable ? (
                <Switch
                  aria-label={`${item.label} on ${computerName}`}
                  checked={allowed}
                  onCheckedChange={next => {
                    onToggleScope?.(item.scope, next);
                    toast.message(
                      next
                        ? `Allowed ${item.label} on ${computerName}.`
                        : `Turned off ${item.label} on ${computerName}.`
                    );
                  }}
                  size="sm"
                />
              ) : (
                <span
                  className={
                    allowed
                      ? "shrink-0 text-meta font-medium text-fg"
                      : "shrink-0 text-meta text-faint"
                  }
                >
                  {allowed ? "Allowed" : "Not granted"}
                </span>
              )}
            </div>
          );
        })}
      </div>

      <p className="text-meta text-muted-foreground">
        {editable
          ? `Turning off a permission takes effect right away — open terminals on ${computerName} close within 2 seconds.`
          : `Controls for missing permissions stay hidden in remote sessions. Narrow or restore permissions on ${computerName} → Settings → Computers → This computer → Devices.`}
      </p>
    </div>
  );
}
