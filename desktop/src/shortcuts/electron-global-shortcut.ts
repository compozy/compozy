import type { GlobalShortcutLike } from "./global-shortcut-policy";

export class ElectronGlobalShortcut implements GlobalShortcutLike {
  readonly #delegate: GlobalShortcutLike;
  readonly #callbacks = new Map<string, () => void>();

  constructor(delegate: GlobalShortcutLike) {
    this.#delegate = delegate;
  }

  register(accelerator: string, callback: () => void): boolean {
    const registered = this.#delegate.register(accelerator, callback);
    if (registered) this.#callbacks.set(accelerator, callback);
    return registered;
  }

  unregister(accelerator: string): void {
    this.#delegate.unregister(accelerator);
    this.#callbacks.delete(accelerator);
  }

  unregisterAll(): void {
    this.#delegate.unregisterAll();
    this.#callbacks.clear();
  }

  invokeForE2E(accelerator: string): boolean {
    const callback = this.#callbacks.get(accelerator);
    if (callback === undefined) return false;
    callback();
    return true;
  }
}
