import { useEffect } from "react";
import { ArrowUpRight, Check, Info, X } from "@compozy/ui/icons";

import { Button, cn, Pill, StatusDot } from "@compozy/ui";

import type { HostedPeerFixture } from "../mocks/remote-computers-fixtures";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface PhoneComputerMenuProps {
  open: boolean;
  currentComputerName: string;
  peers: readonly HostedPeerFixture[];
  /** `sheet` on phone widths (<1024 px); `popover` anchored under the header picker on wide screens. */
  presentation?: "sheet" | "popover";
  onClose: () => void;
  onSelectPeer?: (peer: HostedPeerFixture) => void;
}

/**
 * Computer switcher for the browser SPA on a hosted route (U8, US-022 AC-4).
 *
 * Lists the current computer plus other account computers with Hosted route
 * enabled (`GET /remote/v1/browser/peers`). Clearly signals with `↗` and the
 * peer's hostname that selecting another computer opens its own address.
 */
export function PhoneComputerMenu({
  open,
  currentComputerName,
  peers,
  presentation = "sheet",
  onClose,
  onSelectPeer,
}: PhoneComputerMenuProps) {
  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [open, onClose]);

  if (!open) return null;

  const otherPeers = peers.filter(peer => !peer.current);
  const currentPeer = peers.find(peer => peer.current);
  const isPopover = presentation === "popover";

  return (
    <div
      aria-label="Computers on your account"
      aria-modal="true"
      className={cn(
        "absolute inset-0 z-30 flex",
        isPopover ? "items-start justify-start p-3 pt-14" : "flex-col justify-end"
      )}
      data-presentation={presentation}
      data-slot="phone-computer-menu"
      role="dialog"
    >
      <button
        aria-label="Close computer menu"
        className={cn("absolute inset-0", isPopover ? "bg-overlay-scrim/40" : "bg-overlay-scrim")}
        onClick={onClose}
        tabIndex={-1}
        type="button"
      />
      <div
        className={cn(
          "relative z-10 flex flex-col overflow-hidden bg-popover",
          isPopover
            ? "max-h-[calc(100%-4rem)] w-[336px] rounded-lg border border-line shadow-pop"
            : "max-h-[82%] w-full rounded-t-xl border-t border-line shadow-overlay"
        )}
      >
        {!isPopover ? (
          <div className="flex items-center justify-center pt-2.5 pb-1">
            <span aria-hidden="true" className="h-1 w-9 rounded-pill bg-line-strong" />
          </div>
        ) : null}

        <div className="flex items-center justify-between gap-2 border-b border-line-soft px-4 py-2.5">
          <div className="flex min-w-0 flex-col">
            <h2 className="text-item-title font-medium text-fg-strong">Computers</h2>
            <p className="text-eyebrow text-subtle">Hosted routes on your account</p>
          </div>
          <Button aria-label="Close computer menu" onClick={onClose} size="icon-sm" variant="quiet">
            <X className="size-4" />
          </Button>
        </div>

        <div className="flex flex-col divide-y divide-line-soft overflow-y-auto px-2 py-2">
          {currentPeer ? (
            <button
              className="flex min-h-14 w-full items-center gap-3 rounded-lg bg-selected px-3 py-2.5 text-left transition-colors focus-visible:shadow-focus-ring focus-visible:outline-none"
              onClick={onClose}
              type="button"
            >
              <ComputerPlatformIcon
                iconKind={currentPeer.iconKind}
                platform={currentPeer.platform}
                size="md"
              />
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <div className="flex items-center gap-2">
                  <span className="truncate text-body font-medium text-fg-strong">
                    {currentPeer.name}
                  </span>
                  <Pill form="tint" size="xs" tone="neutral">
                    Current
                  </Pill>
                </div>
                <span className="truncate font-mono text-mono-id text-subtle">
                  {currentPeer.hostname}
                </span>
              </div>
              <Check aria-label="Current computer" className="size-4 shrink-0 text-fg" />
            </button>
          ) : null}

          {otherPeers.length > 0 ? (
            <div className="flex flex-col gap-0.5 pt-2">
              <div className="px-3 pb-1 text-eyebrow text-subtle">Other hosted routes</div>
              {otherPeers.map(peer => (
                <button
                  className={cn(
                    "flex min-h-14 w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none",
                    !peer.online && "opacity-65"
                  )}
                  key={peer.id}
                  onClick={() => onSelectPeer?.(peer)}
                  type="button"
                >
                  <ComputerPlatformIcon
                    iconKind={peer.iconKind}
                    platform={peer.platform}
                    size="md"
                  />
                  <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                    <div className="flex items-center gap-2">
                      <span className="truncate text-body font-medium text-fg">{peer.name}</span>
                      <span className="inline-flex items-center gap-1 text-eyebrow text-subtle">
                        <StatusDot size="sm" tone={peer.online ? "success" : "faint"} />
                        {peer.online ? "Online" : "Offline"}
                      </span>
                    </div>
                    <span className="truncate font-mono text-mono-id text-subtle">
                      {peer.hostname}
                    </span>
                  </div>
                  <span
                    aria-hidden="true"
                    className="inline-flex size-7 shrink-0 items-center justify-center rounded-md text-subtle"
                  >
                    <ArrowUpRight className="size-4" />
                  </span>
                </button>
              ))}
            </div>
          ) : (
            <div className="mt-2 rounded-lg bg-sunken px-3.5 py-3 text-small-body text-muted-foreground">
              <p className="font-medium text-fg">Only {currentComputerName} is reachable here</p>
              <p className="mt-1 text-eyebrow leading-relaxed text-subtle">
                Only computers with Hosted route turned on appear in this menu. Turn on Hosted route
                in Settings → Computers on another computer to reach it from your phone.
              </p>
            </div>
          )}
        </div>

        <div className="flex items-start gap-2 border-t border-line-soft bg-sunken/70 px-4 py-3 text-eyebrow text-subtle">
          <Info aria-hidden="true" className="mt-0.5 size-3.5 shrink-0 text-faint" />
          <p className="leading-relaxed">
            This browser connects directly to{" "}
            <span className="font-medium text-muted-foreground">{currentComputerName}</span>.
            Choosing another computer opens its own hosted address.
          </p>
        </div>
      </div>
    </div>
  );
}
