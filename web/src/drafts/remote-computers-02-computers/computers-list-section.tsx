import { useState } from "react";
import {
  CloudOff,
  Info,
  Laptop,
  Plus,
  RefreshCw,
  Sparkles,
  TriangleAlert,
} from "@compozy/ui/icons";

import {
  Alert,
  AlertDescription,
  AlertTitle,
  Button,
  Empty,
  PillGroup,
  SearchInput,
} from "@compozy/ui";

import { PlanUsageMeter } from "@/systems/account/components/plan-usage-meter";
import { ComputerRow } from "@/systems/computers/components/computer-row";
import type {
  AccountStateFixture,
  ComputerFixture,
  UpgradeMigrationNoticeFixture,
} from "@/systems/computers/mocks/remote-computers-fixtures";
import { SettingsGroup } from "@/systems/settings/components/settings-group";

type ComputerFilter = "all" | "connected" | "offline" | "disabled";

function filterComputers(
  computers: readonly ComputerFixture[],
  query: string,
  filter: ComputerFilter
): ComputerFixture[] {
  const normalized = query.trim().toLowerCase();
  return computers.filter(computer => {
    if (filter === "connected" && computer.status !== "connected") return false;
    if (filter === "offline" && computer.status !== "offline") return false;
    if (filter === "disabled" && computer.status !== "disabled") return false;
    if (!normalized) return true;
    return `${computer.name} ${computer.sshTarget ?? ""} ${computer.routeInUse?.address ?? ""}`
      .toLowerCase()
      .includes(normalized);
  });
}

export interface ComputersBannersProps {
  account: AccountStateFixture;
  upgradeNotice: UpgradeMigrationNoticeFixture | null;
  uninstallWarning: string | null;
  onReviewWebhooks: () => void;
  onLearnMoreUpgrade: () => void;
  onDismissUpgradeNotice: () => void;
  onRetryCloud: () => void;
  onDismissUninstallWarning: () => void;
}

/** Top-of-page notices for Settings → Computers: U10 upgrade notice, cloud unreachable, uninstall partial failure. */
export function ComputersBanners({
  account,
  upgradeNotice,
  uninstallWarning,
  onReviewWebhooks,
  onLearnMoreUpgrade,
  onDismissUpgradeNotice,
  onRetryCloud,
  onDismissUninstallWarning,
}: ComputersBannersProps) {
  return (
    <>
      {upgradeNotice ? (
        <Alert className="gap-y-1 py-2.5" variant="info">
          <Info aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Remote access was replaced by Computers</span>
            <div className="flex shrink-0 items-center gap-1.5">
              {upgradeNotice.bindingsNeedingConfirmation > 0 ? (
                <Button onClick={onReviewWebhooks} size="xs" type="button" variant="secondary">
                  Review
                </Button>
              ) : null}
              <Button onClick={onLearnMoreUpgrade} size="xs" type="button" variant="ghost">
                Learn more
              </Button>
              <Button onClick={onDismissUpgradeNotice} size="xs" type="button" variant="ghost">
                Dismiss
              </Button>
            </div>
          </AlertTitle>
          <AlertDescription>
            <p>
              Paired devices and connection profiles were removed. Set up remote computers again.
              {upgradeNotice.bindingsNeedingConfirmation > 0 ? (
                <>
                  {" "}
                  <span className="font-medium text-fg">
                    {upgradeNotice.bindingsNeedingConfirmation} webhook triggers need confirmation
                    for their new address.
                  </span>
                </>
              ) : null}
            </p>
          </AlertDescription>
        </Alert>
      ) : null}

      {account.cloudUnreachable ? (
        <Alert className="gap-y-1 py-2.5" variant="warning">
          <CloudOff aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Can't reach Compozy Cloud</span>
            <Button onClick={onRetryCloud} size="xs" type="button" variant="secondary">
              <RefreshCw aria-hidden="true" />
              Try again
            </Button>
          </AlertTitle>
          <AlertDescription>
            Local work is unaffected. Computers reachable over LAN or SSH stay connected.
          </AlertDescription>
        </Alert>
      ) : null}

      {uninstallWarning ? (
        <Alert className="gap-y-1 py-2.5" variant="warning">
          <TriangleAlert aria-hidden="true" />
          <AlertTitle className="flex flex-wrap items-center justify-between gap-2">
            <span>Removed locally; uninstall didn't finish</span>
            <Button onClick={onDismissUninstallWarning} size="xs" type="button" variant="ghost">
              Dismiss
            </Button>
          </AlertTitle>
          <AlertDescription>{uninstallWarning}</AlertDescription>
        </Alert>
      ) : null}
    </>
  );
}

export interface ComputersListSectionProps {
  account: AccountStateFixture;
  computers: readonly ComputerFixture[];
  selectedComputerId?: string | null;
  onAddComputer: () => void;
  onUpgrade: () => void;
  onOpenDetails: (computer: ComputerFixture, tab?: "status" | "providers" | "update") => void;
  onRemove: (computer: ComputerFixture) => void;
  onRemoveRoute: (computer: ComputerFixture, routeId: string) => void;
  onRename: (computer: ComputerFixture) => void;
  onToggleEnabled: (computer: ComputerFixture, nextEnabled: boolean) => void;
}

/** Block 2 of Settings → Computers: thin header quota meter, optional 10+ filter bar, empty state, and `ComputerRow` list. */
export function ComputersListSection({
  account,
  computers,
  selectedComputerId = null,
  onAddComputer,
  onUpgrade,
  onOpenDetails,
  onRemove,
  onRemoveRoute,
  onRename,
  onToggleEnabled,
}: ComputersListSectionProps) {
  const [query, setQuery] = useState("");
  const [filter, setFilter] = useState<ComputerFilter>("all");

  const showFilterBar = computers.length >= 10;
  const visibleComputers = showFilterBar ? filterComputers(computers, query, filter) : computers;
  const needsProCount = computers.filter(item => item.status === "needs_pro").length;
  const activeCount = computers.filter(
    item => item.status !== "needs_pro" && item.status !== "disabled"
  ).length;
  const showQuotaMeter =
    account.signedIn && ((account.plan === "free" && computers.length > 0) || needsProCount > 0);

  return (
    <SettingsGroup
      action={
        showQuotaMeter ? (
          <PlanUsageMeter
            action={
              <Button onClick={onUpgrade} size="xs" type="button" variant="secondary">
                <Sparkles aria-hidden="true" />
                Upgrade
              </Button>
            }
            compact
            label="Remote computers"
            limit={account.remoteComputersLimit}
            needsProCount={needsProCount}
            unitLabel="remote computers"
            used={needsProCount > 0 ? Math.max(1, activeCount) : account.remoteComputersUsed}
          />
        ) : undefined
      }
      description={needsProCount > 0 ? "Pro ended. 1 computer stays active on Free." : undefined}
      title={computers.length > 0 ? `Computers (${computers.length})` : "Computers"}
    >
      {showFilterBar ? (
        <div className="flex flex-wrap items-center justify-between gap-2.5 border-b border-line-soft px-4 py-2.5">
          <SearchInput
            aria-label="Filter computers"
            containerClassName="w-60 bg-canvas"
            kbd={null}
            onChange={setQuery}
            placeholder="Filter computers"
            value={query}
          />
          <PillGroup
            aria-label="Filter by status"
            items={[
              { value: "all", label: `All ${computers.length}` },
              {
                value: "connected",
                label: `Connected ${computers.filter(c => c.status === "connected").length}`,
              },
              {
                value: "offline",
                label: `Offline ${computers.filter(c => c.status === "offline").length}`,
              },
              {
                value: "disabled",
                label: `Turned off ${computers.filter(c => c.status === "disabled").length}`,
              },
            ]}
            onChange={next => setFilter(next as ComputerFilter)}
            size="sm"
            value={filter}
          />
        </div>
      ) : null}

      {computers.length === 0 ? (
        <Empty
          action={
            <Button onClick={onAddComputer} size="sm" type="button">
              <Plus aria-hidden="true" />
              Add computer
            </Button>
          }
          className="px-6 py-10"
          description="Add one over SSH or link one to your account."
          fill={false}
          icon={Laptop}
          title="No other computers yet"
        />
      ) : visibleComputers.length === 0 ? (
        <Empty
          action={
            <Button
              onClick={() => {
                setQuery("");
                setFilter("all");
              }}
              size="sm"
              type="button"
              variant="outline"
            >
              Clear filter
            </Button>
          }
          className="px-6 py-8"
          description={`No computers match "${query.trim()}".`}
          fill={false}
          icon={Laptop}
          size="compact"
          title="Nothing matches"
        />
      ) : (
        visibleComputers.map(computer => (
          <ComputerRow
            computer={computer}
            key={computer.id}
            onOpenDetails={onOpenDetails}
            onRemove={onRemove}
            onRemoveRoute={onRemoveRoute}
            onRename={onRename}
            onToggleEnabled={onToggleEnabled}
            onUpgrade={onUpgrade}
            selected={selectedComputerId === computer.id}
          />
        ))
      )}
    </SettingsGroup>
  );
}
