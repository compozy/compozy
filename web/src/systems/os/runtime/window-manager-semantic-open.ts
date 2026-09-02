import type {
  OsDesktopRuntimeStore,
  OsNavigateMode,
  OsOpenTarget,
  OsWindowRoute,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "../lib/os-types";
import { mruWindowInstance } from "../lib/window-instance-lookup";
import { sameOsWindowRoute } from "../lib/window-manager-route";

interface SemanticOpenDelegate {
  bindingKey(): string;
  getState(): OsDesktopRuntimeStore;
  navigate(
    windowId: string,
    route: OsWindowRoute,
    mode?: OsNavigateMode
  ): WindowManagerCommandOutcome;
  open(target: OsOpenTarget): WindowManagerOpenOutcome;
}

interface PendingSemanticOpen {
  bindingKey: string;
  outcome: WindowManagerOpenOutcome;
  target: OsOpenTarget;
}

/** Serializes the semantic decision around the runtime's serialized command transport. */
export class WindowManagerSemanticOpenCoordinator {
  private readonly pending = new Map<string, PendingSemanticOpen>();

  constructor(private readonly delegate: SemanticOpenDelegate) {}

  reset(): void {
    this.pending.clear();
  }

  openOrFocus(target: OsOpenTarget): WindowManagerOpenOutcome {
    if (target.forceNewInstance || target.stackTargetWindowId) return this.delegate.open(target);
    const bindingKey = this.delegate.bindingKey();
    const key = this.key(bindingKey, target);
    const pending = this.pending.get(key);
    if (pending) return this.follow(pending, target);
    const outcome = this.delegate.open(target);
    if (!outcome.accepted) return outcome;
    let tracked: PendingSemanticOpen;
    const completion = outcome.completion.finally(() => {
      if (this.pending.get(key) === tracked) this.pending.delete(key);
    });
    tracked = { bindingKey, target, outcome: { ...outcome, completion } };
    this.pending.set(key, tracked);
    return tracked.outcome;
  }

  private key(bindingKey: string, target: OsOpenTarget): string {
    return JSON.stringify([bindingKey, target.app, target.instanceKey ?? null]);
  }

  private follow(pending: PendingSemanticOpen, target: OsOpenTarget): WindowManagerOpenOutcome {
    if (
      target.route === undefined ||
      (pending.target.route !== undefined && sameOsWindowRoute(pending.target.route, target.route))
    ) {
      return pending.outcome;
    }
    return {
      windowId: pending.outcome.windowId,
      accepted: true,
      completion: pending.outcome.completion.then(applied =>
        this.continueAfterPending(applied, pending.bindingKey, target)
      ),
    };
  }

  private async continueAfterPending(
    applied: boolean,
    bindingKey: string,
    target: OsOpenTarget
  ): Promise<boolean> {
    if (this.delegate.bindingKey() !== bindingKey) return false;
    if (!applied) return await this.retry(bindingKey, target);
    const state = this.delegate.getState();
    const existing = mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
      app: target.app,
      instanceKey: target.instanceKey ?? null,
    });
    if (existing === null) return await this.retry(bindingKey, target);
    if (target.route === undefined || sameOsWindowRoute(existing.route, target.route)) return true;
    const navigation = this.delegate.navigate(existing.id, target.route, target.navigateMode);
    return navigation.accepted && (await navigation.completion);
  }

  private async retry(bindingKey: string, target: OsOpenTarget): Promise<boolean> {
    if (this.delegate.bindingKey() !== bindingKey) return false;
    const retry = this.openOrFocus(target);
    return retry.accepted && (await retry.completion);
  }
}
