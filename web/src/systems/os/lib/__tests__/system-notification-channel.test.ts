// Suite: system notification channel
// Invariant: the channel reports its real platform state — unsupported, default,
// denied, or granted — and is armed only when the operator opted in AND the
// platform granted permission. No pretend-armed state, and no notification
// fires from an unarmed channel.
// Owning layer: unit (systems/os/lib) — the platform capability boundary.
import { afterEach, describe, expect, it, vi } from "vitest";

import {
  requestSystemNotifications,
  showSystemNotification,
  systemNotificationsArmed,
  systemNotificationsSupported,
  systemNotificationState,
} from "../system-notification-channel";

interface FakeNotification {
  onclick: (() => void) | null;
  close: () => void;
}

function stubNotification(
  permission: NotificationPermission,
  options: { requestResult?: NotificationPermission; construct?: () => never } = {}
) {
  const instances: FakeNotification[] = [];
  // A real class, not `vi.fn`: the production code constructs this with `new`,
  // and an arrow-function mock is not constructible.
  class StubNotification implements FakeNotification {
    onclick: (() => void) | null = null;
    close = vi.fn();
    constructor(
      readonly title: string,
      readonly init?: NotificationOptions
    ) {
      if (options.construct) options.construct();
      instances.push(this);
    }
  }
  const requestPermission = vi.fn(async () => {
    const next = options.requestResult ?? permission;
    Object.assign(StubNotification, { permission: next });
    return next;
  });
  Object.assign(StubNotification, { permission, requestPermission });
  vi.stubGlobal("Notification", StubNotification);
  return { constructor: StubNotification, instances };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("system notification state (UT-070)", () => {
  it("Should report the channel unsupported when the platform has no Notification API", () => {
    // A control must never render for a capability the platform lacks.
    vi.stubGlobal("Notification", undefined);
    expect(systemNotificationState()).toBe("unsupported");
    expect(systemNotificationsSupported()).toBe(false);
    expect(systemNotificationsArmed(true)).toBe(false);
  });

  it("Should report denied with the switch left unarmed", () => {
    stubNotification("denied");
    expect(systemNotificationState()).toBe("denied");
    expect(systemNotificationsSupported()).toBe(true);
    expect(systemNotificationsArmed(true)).toBe(false);
  });

  it("Should treat an unasked permission as not armed", () => {
    stubNotification("default");
    expect(systemNotificationState()).toBe("default");
    expect(systemNotificationsArmed(true)).toBe(false);
  });

  it("Should arm only when the operator opted in and permission is granted", () => {
    stubNotification("granted");
    expect(systemNotificationState()).toBe("granted");
    expect(systemNotificationsArmed(true)).toBe(true);
    // Permission alone is not consent — the setting is still off.
    expect(systemNotificationsArmed(false)).toBe(false);
  });
});

describe("permission flow (UT-070)", () => {
  it("Should return the state the platform actually left behind", async () => {
    stubNotification("default", { requestResult: "granted" });
    await expect(requestSystemNotifications()).resolves.toBe("granted");
  });

  it("Should report denied truthfully when the operator refuses", async () => {
    stubNotification("default", { requestResult: "denied" });
    await expect(requestSystemNotifications()).resolves.toBe("denied");
  });

  it("Should short-circuit on an unsupported platform", async () => {
    vi.stubGlobal("Notification", undefined);
    await expect(requestSystemNotifications()).resolves.toBe("unsupported");
  });
});

describe("firing (UT-070)", () => {
  it("Should fire and activate through the supplied handler", () => {
    const { instances } = stubNotification("granted");
    const onActivate = vi.fn();
    vi.stubGlobal("focus", vi.fn());

    expect(showSystemNotification({ title: "Chargeback flow", tag: "k", onActivate })).toBe(true);
    instances[0]?.onclick?.();

    expect(onActivate).toHaveBeenCalledTimes(1);
  });

  it("Should refuse to fire from an unarmed channel", () => {
    stubNotification("denied");
    expect(showSystemNotification({ title: "Chargeback flow", tag: "k" })).toBe(false);
  });

  it("Should report failure instead of throwing when the platform rejects the call", () => {
    stubNotification("granted", {
      construct: () => {
        throw new Error("blocked");
      },
    });
    expect(showSystemNotification({ title: "Chargeback flow", tag: "k" })).toBe(false);
  });
});
