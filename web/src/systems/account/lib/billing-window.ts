/**
 * Browser tab opener for Stripe Checkout and Customer Portal sessions.
 *
 * The Stripe URL only exists after an async POST to the daemon, and browsers
 * block `window.open` called outside the click gesture. Reserving a blank tab
 * synchronously during the click preserves user-gesture permission; on the
 * Electron desktop shell (which denies blank in-app windows and hands URLs to
 * the OS browser), the reservation comes back null and `navigate` opens the
 * URL directly.
 */
export interface BillingBrowserWindow {
  /** Sends the reserved tab (or a new tab) to the Stripe page. */
  navigate: (url: string) => void;
  /** Closes the reserved tab when checkout or portal creation fails. */
  close: () => void;
}

export function openBillingBrowserWindow(): BillingBrowserWindow {
  const reserved = typeof window !== "undefined" ? window.open("", "_blank") : null;
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
      if (typeof window !== "undefined") {
        window.open(url, "_blank", "noopener,noreferrer");
      }
    },
    close: () => {
      if (settled) return;
      settled = true;
      if (reserved && !reserved.closed) {
        reserved.close();
      }
    },
  };
}

/** Reopens an already-created Stripe Checkout URL directly from a click. */
export function reopenBillingUrl(url: string): void {
  if (typeof window !== "undefined") {
    window.open(url, "_blank", "noopener,noreferrer");
  }
}
