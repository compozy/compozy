import { useState } from "react";
import { ChevronDown } from "@compozy/ui/icons";

import { cn, Popover, PopoverContent, PopoverTrigger, Spinner, StatusDot } from "@compozy/ui";

import type { ComputerFixture, HostedPeerFixture } from "../mocks/remote-computers-fixtures";
import { ComputerPlatformIcon } from "./computer-platform-icon";
import { PhoneComputerMenu } from "./phone-computer-menu";

export interface HostedComputerMenuProps {
  /** The computer serving this browser tab (its hosted address). */
  computer: ComputerFixture;
  peers: readonly HostedPeerFixture[];
  /** The page lost its connection to `computer` and is reconnecting. */
  reconnecting?: boolean;
  defaultOpen?: boolean;
  onSelectPeer?: (peer: HostedPeerFixture) => void;
  onSignOut?: () => void;
  className?: string;
}

/**
 * Header computer picker of the CompozyOS web app opened on a hosted route (U8, US-022 AC-4):
 * names the computer that serves this tab and opens the account's other hosted routes and
 * **Sign out**. Same footprint as the session `ComputerChip`, so it sits in the window topbar.
 */
export function HostedComputerMenu({
  computer,
  peers,
  reconnecting = false,
  defaultOpen = false,
  onSelectPeer,
  onSignOut,
  className,
}: HostedComputerMenuProps) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <Popover onOpenChange={setOpen} open={open}>
      <PopoverTrigger
        aria-label={`Connected to ${computer.name}${reconnecting ? ", reconnecting" : ""}. Switch computer`}
        className={cn(
          "inline-flex h-6.5 max-w-48 shrink-0 cursor-pointer items-center gap-1.5 rounded-md border border-line bg-surface-2/60 px-2 text-eyebrow text-fg-2 transition-colors duration-base hover:border-line-strong hover:bg-surface-2 hover:text-fg focus-visible:shadow-focus-ring focus-visible:outline-none",
          className
        )}
        data-slot="hosted-computer-menu"
        type="button"
      >
        <ComputerPlatformIcon
          className="size-3.5 shrink-0"
          iconKind={computer.iconKind}
          platform={computer.platform}
          size="sm"
          tone="muted"
        />
        <span className="truncate font-medium">{computer.name}</span>
        {reconnecting ? (
          <Spinner aria-hidden="true" className="size-2.5 text-warning" />
        ) : (
          <StatusDot size="sm" tone="success" />
        )}
        <ChevronDown aria-hidden="true" className="size-3 shrink-0 text-subtle" />
      </PopoverTrigger>

      <PopoverContent align="start" className="w-[336px] overflow-hidden p-0" sideOffset={6}>
        <PhoneComputerMenu
          currentComputerName={computer.name}
          onClose={() => setOpen(false)}
          onSelectPeer={peer => {
            setOpen(false);
            onSelectPeer?.(peer);
          }}
          onSignOut={() => {
            setOpen(false);
            onSignOut?.();
          }}
          open
          peers={peers}
          presentation="panel"
        />
      </PopoverContent>
    </Popover>
  );
}
