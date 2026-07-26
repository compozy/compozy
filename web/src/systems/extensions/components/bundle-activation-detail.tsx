import { Link, useNavigate } from "@tanstack/react-router";
import { Bot, Box, Clock3, Link2, Radio, Zap } from "lucide-react";
import type { ReactNode } from "react";

import {
  Button,
  cn,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Empty,
  ListingRow,
  MonoId,
  PAGE_CONTENT_GUTTER,
  Pill,
  Section,
  Skeleton,
  Switch,
  Time,
  TopbarOverflowIcon,
  useTopbarSlot,
} from "@agh/ui";

import { DeactivateBundleDialog } from "./extension-dialogs";
import { useBundleActivationDetail } from "../hooks/use-bundle-activation-detail";
import type { BundleActivation } from "../types";

export function BundleActivationDetail({ id }: { id: string }) {
  const {
    activation,
    applyUpdate,
    confirmDeactivate,
    confirmNetworkRequirement,
    deactivate,
    dialogOpen,
    query,
    setConfirmNetworkRequirement,
    setDialogOpen,
    update,
  } = useBundleActivationDetail(id);
  const networkConfirmationRequired = Boolean(
    activation?.network_requirement_digest && !activation.network_requirement_confirmed_by
  );
  const navigate = useNavigate();
  const updateActionRequired = Boolean(activation?.spec_drift) || networkConfirmationRequired;
  const backToMarketplace = () => {
    void navigate({ to: "/marketplace/bundles" });
  };

  useTopbarSlot({
    onBack: backToMarketplace,
    crumbs: [{ id: "marketplace", label: "Marketplace", onSelect: backToMarketplace }],
    crumb: activation?.bundle_name ?? id,
    status: activation ? (
      <Pill tone={updateActionRequired ? "warning" : "success"}>
        <Pill.Dot tone={updateActionRequired ? "warning" : "success"} />
        {updateActionRequired ? "Update available" : "Active"}
      </Pill>
    ) : undefined,
    actions: activation ? (
      updateActionRequired ? (
        <Button disabled={update.isPending} onClick={applyUpdate} size="sm">
          Update
        </Button>
      ) : (
        <Button
          render={<Link search={{ q: activation.bundle_name }} to="/marketplace/bundles" />}
          nativeButton={false}
          size="sm"
        >
          View in marketplace
        </Button>
      )
    ) : undefined,
    overflow: activation ? (
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label={`Actions for ${activation.bundle_name}`}
          render={<Button size="icon-sm" type="button" variant="ghost" />}
        >
          <TopbarOverflowIcon aria-hidden="true" className="size-3" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end">
          {updateActionRequired ? (
            <DropdownMenuItem
              render={<Link search={{ q: activation.bundle_name }} to="/marketplace/bundles" />}
            >
              View in marketplace
            </DropdownMenuItem>
          ) : null}
          <DropdownMenuItem variant="destructive" onClick={() => setDialogOpen(true)}>
            Deactivate…
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    ) : undefined,
  });

  if (query.isLoading) return <BundleActivationDetailSkeleton />;
  if (query.error)
    return (
      <Empty
        description={query.error.message}
        icon={Box}
        title="Unable to load bundle activation"
      />
    );
  if (!activation) return <Empty icon={Box} title="Bundle activation not found" />;

  const capabilityCount = (activation.inventory ?? []).length;
  return (
    <div className="min-h-0 flex-1 overflow-y-auto" data-testid="bundle-activation-detail">
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col")}>
        <div className="flex flex-col gap-2 pt-4" data-testid="bundle-activation-detail-header">
          <div className="flex flex-wrap items-center gap-1.5">
            <Pill mono size="xs">
              {activation.profile_name}
            </Pill>
            <Pill mono size="xs">
              {activation.scope}
            </Pill>
          </div>
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-small-body text-subtle">
            <span>{capabilityCount} capabilities</span>
            <span aria-hidden="true">·</span>
            <span>
              activated <Time iso={activation.created_at} />
            </span>
          </div>
        </div>
        <div className="grid gap-8 py-7 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <main className="space-y-7">
            <Description activation={activation} />
            <ResourceSection activation={activation} kind="agents" />
            <ResourceSection activation={activation} kind="jobs" />
            <ResourceSection activation={activation} kind="triggers" />
            <ResourceSection activation={activation} kind="bridges" />
            <ResourceSection activation={activation} kind="channels" />
            <Section count={capabilityCount} label="Inventory">
              <div className="overflow-hidden rounded-lg border border-line">
                {(activation.inventory ?? []).length ? (
                  activation.inventory?.map(item => (
                    <ListingRow
                      interactive={false}
                      key={`${item.resource_kind}:${item.resource_id}`}
                    >
                      <ListingRow.Icon>
                        <Box className="size-4" />
                      </ListingRow.Icon>
                      <ListingRow.Main>
                        <ListingRow.Name mono>
                          <ListingRow.Title>{item.resource_name}</ListingRow.Title>
                        </ListingRow.Name>
                        <ListingRow.Meta>
                          <MonoId value={item.resource_id} />
                        </ListingRow.Meta>
                      </ListingRow.Main>
                      <ListingRow.Trail>
                        <Pill mono size="xs">
                          {item.resource_kind}
                        </Pill>
                      </ListingRow.Trail>
                    </ListingRow>
                  ))
                ) : (
                  <div className="px-4 py-4 text-small-body text-muted">No inventory entries.</div>
                )}
              </div>
            </Section>
          </main>
          <aside className="space-y-7">
            <InfoRail label="Scope">
              <InfoRailRow term="Scope">
                <Pill mono size="xs">
                  {activation.scope}
                </Pill>
              </InfoRailRow>
              <InfoRailRow term="Workspace">
                {activation.workspace_id ? (
                  <MonoId value={activation.workspace_id} />
                ) : (
                  <code className="font-mono text-xs text-fg">—</code>
                )}
              </InfoRailRow>
            </InfoRail>
            {activation.network_requirement_digest ? (
              <InfoRail label="Network requirement">
                <InfoRailRow term="Participation">
                  <Pill tone={activation.network_requirement_confirmed_by ? "success" : "warning"}>
                    {activation.network_requirement_confirmed_by
                      ? "Live confirmed"
                      : "Confirmation required"}
                  </Pill>
                </InfoRailRow>
                <InfoRailRow term="Digest">
                  <MonoId value={activation.network_requirement_digest} />
                </InfoRailRow>
                {activation.network_requirement_confirmed_by ? (
                  <InfoRailRow term="Confirmed by">
                    <code className="font-mono text-xs text-fg">
                      {activation.network_requirement_confirmed_by}
                    </code>
                  </InfoRailRow>
                ) : null}
                {activation.network_requirement_confirmed_at ? (
                  <InfoRailRow term="Confirmed at">
                    <Time iso={activation.network_requirement_confirmed_at} />
                  </InfoRailRow>
                ) : null}
              </InfoRail>
            ) : null}
            {updateActionRequired ? (
              <Section label="Update confirmation">
                <div className="flex items-start gap-3 rounded-lg bg-canvas-soft px-4 py-3">
                  <Switch
                    aria-describedby="bundle-update-network-confirmation-description"
                    checked={confirmNetworkRequirement}
                    disabled={update.isPending}
                    id="bundle-update-network-confirmation"
                    onCheckedChange={setConfirmNetworkRequirement}
                    size="sm"
                  />
                  <span className="flex flex-col gap-1">
                    <label
                      className="text-small-body text-fg"
                      htmlFor="bundle-update-network-confirmation"
                    >
                      Confirm Live network participation
                    </label>
                    <span
                      className="text-xs text-muted"
                      id="bundle-update-network-confirmation-description"
                    >
                      Required only if the updated extension declares Live AGH Network
                      participation.
                    </span>
                  </span>
                </div>
              </Section>
            ) : null}
            <InfoRail label="Timestamps">
              <InfoRailRow term="Created">
                <Time iso={activation.created_at} />
              </InfoRailRow>
              <InfoRailRow term="Updated">
                <Time iso={activation.updated_at} />
              </InfoRailRow>
            </InfoRail>
          </aside>
        </div>
      </div>
      <DeactivateBundleDialog
        activation={activation}
        error={deactivate.error?.message}
        onConfirm={confirmDeactivate}
        onOpenChange={setDialogOpen}
        open={dialogOpen}
        pending={deactivate.isPending}
      />
    </div>
  );
}

function BundleActivationDetailSkeleton() {
  return (
    <div className="min-h-0 flex-1 overflow-y-auto" role="status">
      <div className={cn(PAGE_CONTENT_GUTTER, "flex flex-col")}>
        <div className="flex flex-col gap-2 border-b border-line py-5">
          <Skeleton className="h-3 w-48" />
          <div className="flex items-start gap-3.5">
            <Skeleton className="size-(--size-provider-logo-well) rounded-lg" />
            <div className="flex min-w-0 flex-1 flex-col gap-2">
              <Skeleton className="h-6 w-40" />
              <Skeleton className="h-3 w-56" />
            </div>
          </div>
        </div>
        <div className="grid gap-8 py-7 lg:grid-cols-[minmax(0,1fr)_20rem]">
          <div className="space-y-7">
            {[0, 1, 2].map(index => (
              <div className="space-y-2.5" key={index}>
                <Skeleton className="h-3 w-24" />
                <Skeleton className="h-16 w-full rounded-lg" />
              </div>
            ))}
          </div>
          <aside className="space-y-7">
            <Skeleton className="h-24 w-full rounded-lg" />
            <Skeleton className="h-20 w-full rounded-lg" />
          </aside>
        </div>
      </div>
      <span className="sr-only">Loading bundle activation</span>
    </div>
  );
}

function Description({ activation }: { activation: BundleActivation }) {
  return (
    <Section label="Description">
      <div className="divide-y divide-line-soft rounded-lg bg-canvas-soft">
        <p className="px-4 py-3 text-small-body text-fg">
          {activation.bundle_description ?? "No bundle description is available."}
        </p>
        <div className="px-4 py-3">
          <b className="text-small-body font-medium text-fg-strong">
            Profile · {activation.profile_name}
          </b>
          <p className="mt-1 text-small-body text-muted">
            {activation.profile_description ?? "No profile description is available."}
          </p>
        </div>
      </div>
    </Section>
  );
}

type ResourceKind = "agents" | "jobs" | "triggers" | "bridges" | "channels";
const RESOURCE_ICON = {
  agents: Bot,
  jobs: Clock3,
  triggers: Zap,
  bridges: Link2,
  channels: Radio,
} as const;

function ResourceSection({
  activation,
  kind,
}: {
  activation: BundleActivation;
  kind: ResourceKind;
}) {
  const rows = activation[kind] ?? [];
  const Icon = RESOURCE_ICON[kind];
  return (
    <Section count={rows.length} label={kind}>
      <div className="overflow-hidden rounded-lg border border-line">
        {rows.length ? (
          rows.map(item => {
            const name = item.name;
            const rowId = "id" in item ? item.id : `${kind}:${name}`;
            return (
              <ListingRow interactive={false} key={rowId}>
                <ListingRow.Icon>
                  <Icon className="size-4" />
                </ListingRow.Icon>
                <ListingRow.Main>
                  <ListingRow.Name mono>
                    <ListingRow.Title>{name}</ListingRow.Title>
                  </ListingRow.Name>
                  {"description" in item && item.description ? (
                    <ListingRow.Description>{item.description}</ListingRow.Description>
                  ) : null}
                  <ListingRow.Meta>
                    {"agent_name" in item ? <span>{item.agent_name}</span> : null}
                    {"platform" in item ? <span>{item.platform}</span> : null}
                  </ListingRow.Meta>
                </ListingRow.Main>
                <ListingRow.Trail>
                  <Pill mono size="xs">
                    {kind.slice(0, -1)}
                  </Pill>
                </ListingRow.Trail>
              </ListingRow>
            );
          })
        ) : (
          <div className="px-4 py-4 text-small-body text-muted">None in this activation.</div>
        )}
      </div>
    </Section>
  );
}

function InfoRail({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Section label={label}>
      <dl className="divide-y divide-line-soft overflow-hidden rounded-lg bg-canvas-soft">
        {children}
      </dl>
    </Section>
  );
}

function InfoRailRow({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 px-4 py-3">
      <dt className="eyebrow text-muted">{term}</dt>
      <dd className="flex min-w-0 flex-wrap items-center gap-1.5 text-xs text-fg">{children}</dd>
    </div>
  );
}
