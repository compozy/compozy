/**
 * The browser tab a sign-in opens in. The sign-in address only exists after an async begin, and a
 * browser blocks a tab opened outside the click, so the click reserves a blank tab and the begin's
 * answer navigates it. The desktop app denies every in-app window and hands web addresses to the
 * system browser instead: there the reservation comes back empty and the address opens directly.
 */
export interface MCPSignInWindow {
  /** Sends the reserved tab (or a new one) to the sign-in page. */
  navigate: (url: string) => void;
  /** Gives back a reserved tab nobody needs (begin failed, sign-in cancelled). */
  close: () => void;
}

export function openMCPSignInWindow(): MCPSignInWindow {
  const reserved = window.open("", "_blank");
  let settled = false;
  return {
    navigate: url => {
      if (settled) return;
      settled = true;
      if (reserved && !reserved.closed) {
        try {
          reserved.opener = null;
          reserved.location.replace(url);
          return;
        } catch {
          reserved.close();
        }
      }
      window.open(url, "_blank", "noopener,noreferrer");
    },
    close: () => {
      if (settled) return;
      settled = true;
      reserved?.close();
    },
  };
}

/** Reopens the sign-in page from a click ("Open again"). */
export function reopenMCPSignInPage(url: string): void {
  window.open(url, "_blank", "noopener,noreferrer");
}
