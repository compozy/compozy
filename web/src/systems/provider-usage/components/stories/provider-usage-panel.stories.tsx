import type { Meta, StoryObj } from "@storybook/react-vite";
import { useRef, type ComponentProps } from "react";
import { fn, userEvent, within } from "storybook/test";

import { Popover, PopoverContent } from "@compozy/ui";

import { PanelSurface } from "@/storybook/story-layout";

import { buildProviderUsageViewModel } from "../../lib/view-model";
import {
  claudeOkTarget,
  codexOkTarget,
  defaultUsageEnvelope,
  edgeCaseTarget,
  statusTargets,
  usageEnvelope,
  usageTarget,
  usageWindow,
} from "../../mocks";
import type { ProviderUsageTarget } from "../../types";
import { ProviderUsagePanel } from "../provider-usage-panel";

const NOW = Date.now();
const STATUS = statusTargets(NOW);

type PanelArgs = ComponentProps<typeof ProviderUsagePanel>;

/** Rows start collapsed; detail stories open the named rows the way a person would. */
function expandRows(...providerNames: string[]): NonNullable<Story["play"]> {
  return async ({ canvasElement }) => {
    const body = within(canvasElement.ownerDocument.body);
    for (const name of providerNames) {
      await userEvent.click(await body.findByRole("button", { name: new RegExp(`^${name}`) }));
    }
  };
}

function rowsOf(targets: ProviderUsageTarget[]) {
  return buildProviderUsageViewModel(usageEnvelope(targets, {}, NOW));
}

/**
 * The S2 content: the panel in an open popover, anchored to a bare point at
 * the top-right corner. The tray trigger and its footprint belong to the OS
 * menubar stories (`OsMenuBar` › Usage popover open).
 */
function PopoverStage(args: PanelArgs) {
  const anchor = useRef<HTMLSpanElement>(null);
  return (
    <div className="flex min-h-160 w-full items-start justify-end bg-rail p-2">
      <span ref={anchor} aria-hidden="true" className="size-8.5" />
      <Popover open>
        <PopoverContent
          anchor={anchor}
          align="end"
          aria-label="Provider usage"
          className="w-80 max-w-[calc(100vw-16px)] p-2"
        >
          <ProviderUsagePanel {...args} />
        </PopoverContent>
      </Popover>
    </div>
  );
}

const meta: Meta<typeof ProviderUsagePanel> = {
  component: ProviderUsagePanel,
  title: "systems/provider-usage/components/ProviderUsagePanel",
  render: args => <PopoverStage {...args} />,
  args: {
    viewModel: buildProviderUsageViewModel(defaultUsageEnvelope(NOW)),
    isMacHost: true,
    onRefresh: fn(),
    onAction: fn(),
    onOpenUsage: fn(),
    onOpenSettings: fn(),
  },
  parameters: { layout: "fullscreen" },
};

export default meta;
type Story = StoryObj<typeof meta>;

/** US-002.AC-1/AC-2: header, freshness, one row per target with its tightest window. */
export const Default: Story = {};

/**
 * US-006.EC-1: first read in flight. The rows hold their place, one collapsed row each, and the
 * header holds the freshness line, so the popover is the size it will be.
 */
export const Loading: Story = { args: { viewModel: null, loading: true } };

/** US-005.AC-1: explicit refresh in flight; rows keep their data. */
export const Refreshing: Story = {
  args: {
    refreshing: true,
    viewModel: rowsOf([{ ...claudeOkTarget(NOW), refreshing: true }, codexOkTarget(NOW)]),
  },
};

/** US-006.AC-1: last-good numbers dimmed, ring marker, original measured time, Retry. */
export const StaleRow: Story = {
  args: { viewModel: rowsOf([claudeOkTarget(NOW), STATUS.stale]) },
};

/** US-007.AC-1, EC-1, EC-2: provider-specific sign-in reasons, no bar, no button. */
export const SignedOut: Story = {
  args: {
    viewModel: rowsOf([STATUS.claudeAuthScope, STATUS.codexAuthExpired, STATUS.signedOut]),
  },
};

/** US-008.AC-1: Claude needs Keychain consent — **Allow** on a macOS host. */
export const NeedsConsent: Story = {
  args: { viewModel: rowsOf([STATUS.needsConsent, codexOkTarget(NOW)]) },
};

/** US-008.EC-1: the macOS prompt was denied — **Try again**. */
export const KeychainDenied: Story = {
  args: { viewModel: rowsOf([STATUS.keychainDenied, codexOkTarget(NOW)]) },
};

/** US-008.AC-5: the loaded login expired — the row says how to reload it. */
export const KeychainTokenExpired: Story = {
  args: { viewModel: rowsOf([STATUS.keychainTokenExpired, codexOkTarget(NOW)]) },
};

/** US-008.EC-3: off macOS the Keychain path does not exist — no **Allow**. */
export const NeedsConsentOffMac: Story = {
  args: { isMacHost: false, viewModel: rowsOf([STATUS.needsConsent, codexOkTarget(NOW)]) },
};

/** US-008.EC-4: a remote client sees where the approval happens. */
export const RemoteConsent: Story = {
  args: { remoteClient: true, viewModel: rowsOf([STATUS.needsConsent, codexOkTarget(NOW)]) },
};

/** US-006.EC-3: another login replaced the old one — the row reloads with no old numbers. */
export const LoginReplacedReloading: Story = {
  args: { viewModel: rowsOf([claudeOkTarget(NOW), STATUS.refreshing]) },
};

/** US-005.EC-2: provider 429 with its cooldown, Retry bypasses it once. */
export const RateLimited: Story = {
  args: { viewModel: rowsOf([STATUS.rateLimited, codexOkTarget(NOW)]) },
};

/** US-007.AC-3/EC-3: a failed read with Retry; the message is in the detail. */
export const ErrorRow: Story = {
  args: { viewModel: rowsOf([claudeOkTarget(NOW), STATUS.error]) },
};

/** US-002.EC-2, US-007.AC-4/EC-4: unsupported and disabled rows are text only. */
export const Unsupported: Story = {
  args: {
    viewModel: rowsOf([codexOkTarget(NOW), STATUS.unsupported, STATUS.noPlan, STATUS.disabled]),
  },
};

/**
 * US-003.EC-1–EC-4 and US-004: unknown percent, no cap reported, two
 * currencies, reset text only, "Resetting…", a date beyond a week, above 100 %.
 */
export const EdgeCasesExpanded: Story = {
  args: { viewModel: rowsOf([edgeCaseTarget(NOW)]) },
  tags: ["play-fn"],
  play: expandRows("OpenRouter"),
};

/** US-003.AC-1–AC-4: Claude expanded — three bars, extra usage, plan and source. */
export const DetailExpanded: Story = {
  args: { viewModel: rowsOf([claudeOkTarget(NOW), codexOkTarget(NOW)]) },
  tags: ["play-fn"],
  play: expandRows("Claude"),
};

/** A signed-out row opened: the reason sits under the row, never in it. */
export const SignedOutExpanded: Story = {
  args: SignedOut.args,
  tags: ["play-fn"],
  play: expandRows("Cursor"),
};

/** US-005.EC-3: daemon unreachable — cached rows under a banner, Refresh absent. */
export const DaemonUnavailable: Story = { args: { daemonUnavailable: true } };

/**
 * US-002.EC-1: ten or more targets scroll under a fixed header. The overlay
 * bar shows on hover or scroll, over a gutter that keeps it off the rows.
 */
export const ManyTargets: Story = {
  args: {
    viewModel: rowsOf([
      ...Array.from({ length: 12 }, (_, index) =>
        usageTarget({
          provider_id: `provider-${index}`,
          provider_name: `Provider ${index + 1}`,
          measured_at: new Date(NOW - 60_000).toISOString(),
          windows: [
            usageWindow({
              id: "monthly",
              label: "Monthly",
              used_percent: (index * 9) % 105,
              resets_at: new Date(NOW + (index + 1) * 3_600_000).toISOString(),
            }),
          ],
        })
      ),
      STATUS.signedOut,
      STATUS.codexAuthExpired,
      STATUS.claudeAuthScope,
    ]),
  },
};

/** US-002.EC-4: the popover clamps to a viewport narrower than its width. */
export const NarrowViewport: Story = {
  parameters: { viewport: { defaultViewport: "mobile1" } },
};

/** The feature was turned off while the panel was open. */
export const TurnedOff: Story = {
  args: {
    viewModel: buildProviderUsageViewModel(usageEnvelope([], { enabled: false }, NOW)),
  },
};

/** S3 / US-010.AC-2: the Usage app — window chrome, every target a collapsed row. */
export const UsageAppWindow: Story = {
  render: args => (
    <PanelSurface className="p-4">
      <ProviderUsagePanel {...args} className="w-full max-w-2xl" />
    </PanelSurface>
  ),
  args: {
    chrome: "window",
    title: "Usage",
    onOpenUsage: undefined,
    viewModel: rowsOf([claudeOkTarget(NOW), codexOkTarget(NOW), STATUS.unsupported]),
  },
};

/** The Usage app before its first read arrives: the same rows' shape in the window chrome. */
export const UsageAppWindowLoading: Story = {
  render: UsageAppWindow.render,
  args: { ...UsageAppWindow.args, viewModel: null, loading: true },
};
