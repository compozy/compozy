import { RefreshCw, WifiOff } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

export interface RemoteOfflineComposerBannerProps {
  computerName: string;
  onReconnect?: () => void;
}

/**
 * Remote-session offline banner shown above the read-only composer when the
 * session's computer is offline (`_uiux.md` U6). Keeps the last loaded
 * transcript readable while explaining why prompts cannot be sent.
 */
export function RemoteOfflineComposerBanner({
  computerName,
  onReconnect,
}: RemoteOfflineComposerBannerProps) {
  return (
    <div
      className="mx-auto flex w-full max-w-3xl items-center gap-3 px-6 pt-2 pb-5"
      data-testid="remote-offline-composer-banner"
      role="status"
    >
      <div className="flex min-w-0 flex-1 items-center gap-3 rounded-lg border border-line bg-surface-2 px-3 py-2.5">
        <WifiOff aria-hidden="true" className="size-4 shrink-0 text-muted-foreground" />
        <span className="min-w-0 flex-1 text-small-body text-muted-foreground">
          {computerName} is offline. Showing the last messages loaded in this session.
        </span>
        {onReconnect ? (
          <Button onClick={onReconnect} size="xs" type="button" variant="secondary">
            <RefreshCw aria-hidden="true" data-icon="inline-start" />
            Reconnect
          </Button>
        ) : null}
      </div>
    </div>
  );
}
