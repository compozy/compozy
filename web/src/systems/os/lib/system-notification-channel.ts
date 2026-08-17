/**
 * The system (OS) notification channel and its truthful platform state.
 *
 * The rule this module exists to keep: no control renders for a channel the
 * platform does not support, and the setting never claims to be armed when it
 * is not. A denied permission is a rendered state with a way forward, not an
 * error and not a silent no-op — so the operator is never left believing alerts
 * will reach them when they cannot.
 */

export type SystemNotificationState =
  /** No Notification API here (some browsers, insecure contexts). */
  | "unsupported"
  /** Supported, permission never asked. Enabling walks the platform flow. */
  | "default"
  /** Supported, permission refused. In-app delivery continues. */
  | "denied"
  /** Supported and granted — the only state that may claim to be armed. */
  | "granted";

export interface SystemNotificationOptions {
  title: string;
  body?: string;
  /** Collapses repeats of the same subject in the platform's own stack. */
  tag?: string;
  onActivate?: () => void;
}

export function systemNotificationState(): SystemNotificationState {
  // `typeof`, not `in`: an insecure context can leave the property present but
  // undefined, and reading `.permission` off it would throw.
  if (typeof window === "undefined" || typeof window.Notification === "undefined") {
    return "unsupported";
  }
  const permission = window.Notification.permission;
  if (permission === "granted") return "granted";
  if (permission === "denied") return "denied";
  return "default";
}

export function systemNotificationsSupported(): boolean {
  return systemNotificationState() !== "unsupported";
}

/** Only a granted permission arms the channel — `default` is not armed. */
export function systemNotificationsArmed(enabled: boolean): boolean {
  return enabled && systemNotificationState() === "granted";
}

/**
 * Walks the platform permission flow. Returns the state afterwards so the
 * caller renders what actually happened rather than what it hoped for.
 */
export async function requestSystemNotifications(): Promise<SystemNotificationState> {
  if (systemNotificationState() === "unsupported") return "unsupported";
  try {
    await window.Notification.requestPermission();
  } catch {
    // A browser that rejects the request leaves the permission untouched; the
    // state read below is still the truth.
  }
  return systemNotificationState();
}

/**
 * Fires a system notification. Callers gate on the app being unfocused or
 * hidden — a visible app owns that moment with its toast, and firing both would
 * double-notify (Business Rule 18).
 */
export function showSystemNotification(options: SystemNotificationOptions): boolean {
  if (systemNotificationState() !== "granted") return false;
  try {
    const notification = new window.Notification(options.title, {
      ...(options.body ? { body: options.body } : {}),
      ...(options.tag ? { tag: options.tag } : {}),
    });
    notification.onclick = () => {
      window.focus();
      options.onActivate?.();
      notification.close();
    };
    return true;
  } catch {
    return false;
  }
}
