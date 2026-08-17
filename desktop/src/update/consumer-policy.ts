import type { UpdateOperation } from "./operation-contract";

export type ConsumerDecision =
  | { readonly kind: "ignore" }
  | { readonly kind: "install" }
  | { readonly kind: "recover-install" }
  | { readonly kind: "verify-restart" }
  | { readonly kind: "fail-silent-install"; readonly reason: string }
  | { readonly kind: "wait" };

function deadlineExpired(deadline: string | undefined, now: Date): boolean {
  if (!deadline) return false;
  const timestamp = Date.parse(deadline);
  return Number.isFinite(timestamp) && timestamp <= now.getTime();
}

function normalizedVersion(version: string): string {
  return version.trim().replace(/^v/u, "");
}

export function decideAppUpdateAction(
  operation: UpdateOperation,
  currentVersion: string,
  now: Date
): ConsumerDecision {
  const app = operation.app;
  if (!app || !operation.targets.includes("app")) return { kind: "ignore" };
  if (
    operation.waiting === "waiting-for-app" &&
    operation.active_target === undefined &&
    operation.holder === null &&
    app.phase === "staged"
  ) {
    return { kind: "install" };
  }
  if (operation.active_target === "app" && app.phase === "staged") {
    if (!operation.holder || deadlineExpired(operation.holder.lease_expires_at, now)) {
      return { kind: "recover-install" };
    }
    return { kind: "wait" };
  }
  if (app.phase === "installer-handoff") {
    if (normalizedVersion(currentVersion) === normalizedVersion(app.to_version)) {
      return { kind: "verify-restart" };
    }
    if (deadlineExpired(app.watchdog_deadline, now)) {
      return {
        kind: "fail-silent-install",
        reason: `The app installer did not replace version ${app.from_version}.`,
      };
    }
    return { kind: "wait" };
  }
  if (app.phase === "applying") {
    if (deadlineExpired(app.watchdog_deadline, now)) {
      return {
        kind: "fail-silent-install",
        reason: "The app update did not reach the installer handoff before its deadline.",
      };
    }
    return { kind: "wait" };
  }
  return { kind: "ignore" };
}
