import { ArrowUpRight, Check, Info, LogIn, X } from "@compozy/ui/icons";

import { Button, cn, Pill, Sheet, SheetContent, SheetTitle, StatusDot } from "@compozy/ui";

import type { HostedPeerFixture } from "../mocks/remote-computers-fixtures";
import { ComputerPlatformIcon } from "./computer-platform-icon";

export interface PhoneComputerMenuProps {
  open: boolean;
  currentComputerName: string;
  peers: readonly HostedPeerFixture[];
  /**
   * `sheet` on phone widths (<1024 px); `panel` renders only the menu body, for a `PopoverContent`
   * anchored to the web app's header computer picker on wide screens (`HostedComputerMenu`).
   */
  presentation?: "sheet" | "panel";
  onClose: () => void;
  onSelectPeer?: (peer: HostedPeerFixture) => void;
  onSignOut?: () => void;
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
  onSignOut,
}: PhoneComputerMenuProps) {
  const otherPeers = peers.filter(peer => !peer.current);
  const currentPeer = peers.find(peer => peer.current);
  const isPanel = presentation === "panel";

  const panel = (
    <div
      className={cn(
        "relative z-10 flex flex-col overflow-hidden bg-popover",
        isPanel ? "w-full rounded-[inherit]" : "min-h-0 flex-1"
      )}
      data-presentation={presentation}
      data-slot={isPanel ? "phone-computer-menu" : undefined}
    >
      {!isPanel ? (
        <div className="flex items-center justify-center pt-2.5 pb-1">
          <span aria-hidden="true" className="h-1 w-9 rounded-pill bg-line-strong" />
        </div>
      ) : null}

      <div className="flex items-center justify-between gap-2 border-b border-line-soft px-4 py-2.5">
        <div className="flex min-w-0 flex-col">
          {isPanel ? (
            <h2 className="text-item-title font-medium text-fg-strong">Computers</h2>
          ) : (
            <SheetTitle className="tracking-normal">Computers</SheetTitle>
          )}
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
                <ComputerPlatformIcon iconKind={peer.iconKind} platform={peer.platform} size="md" />
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
          <span className="font-medium text-muted-foreground">{currentComputerName}</span>. Choosing
          another computer opens its own hosted address.
        </p>
      </div>

      <div className="border-t border-line-soft px-2 py-2">
        <button
          className="flex min-h-11 w-full items-center gap-3 rounded-lg px-3 py-2 text-left text-small-body font-medium text-fg transition-colors hover:bg-surface-2 focus-visible:shadow-focus-ring focus-visible:outline-none"
          data-testid="phone-computer-menu-sign-out"
          onClick={onSignOut}
          type="button"
        >
          <LogIn aria-hidden="true" className="size-4 shrink-0 -scale-x-100 text-subtle" />
          <span className="flex min-w-0 flex-1 flex-col">
            <span>Sign out</span>
            <span className="text-eyebrow font-normal text-subtle">
              Ends this browser session on {currentComputerName}
            </span>
          </span>
        </button>
      </div>
    </div>
  );

  if (isPanel) return open ? panel : null;

  return (
    <Sheet
      disablePointerDismissal={false}
      modal
      open={open}
      onOpenChange={next => {
        if (!next) onClose();
      }}
    >
      <SheetContent
        className="max-h-[82%] gap-0 rounded-t-xl border-t border-line"
        data-slot="phone-computer-menu"
        showCloseButton={false}
        side="bottom"
      >
        {panel}
      </SheetContent>
    </Sheet>
  );
}
