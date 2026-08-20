import { chordToAccelerator, UnconvertibleShortcutError } from "./accelerator";
import type {
  AccessibilityStatus,
  GlobalShortcutBinding,
  GlobalShortcutRegistration,
} from "./global-shortcut-types";

export interface GlobalShortcutLike {
  register(accelerator: string, callback: () => void): boolean;
  unregister(accelerator: string): void;
  unregisterAll(): void;
}

interface ActiveBinding {
  chord: string;
  accelerator: string;
}

export class GlobalShortcutPolicy {
  readonly #globalShortcut: GlobalShortcutLike;
  readonly #accessibility: AccessibilityStatus;
  readonly #onInvoke: (commandID: string) => void;
  readonly #active = new Map<string, ActiveBinding>();
  #statuses: GlobalShortcutRegistration[] = [];

  constructor(options: {
    globalShortcut: GlobalShortcutLike;
    accessibility: AccessibilityStatus;
    onInvoke: (commandID: string) => void;
  }) {
    this.#globalShortcut = options.globalShortcut;
    this.#accessibility = options.accessibility;
    this.#onInvoke = options.onInvoke;
  }

  sync(bindings: readonly GlobalShortcutBinding[]): GlobalShortcutRegistration[] {
    const intendedIDs = new Set(bindings.map(binding => binding.command_id));
    for (const [commandID, active] of this.#active) {
      if (intendedIDs.has(commandID)) continue;
      this.#globalShortcut.unregister(active.accelerator);
      this.#active.delete(commandID);
    }

    this.#statuses = bindings.map(binding => this.#register(binding));
    return this.status();
  }

  status(): GlobalShortcutRegistration[] {
    return this.#statuses.map(status => ({ ...status }));
  }

  unregisterAll(): void {
    this.#globalShortcut.unregisterAll();
    this.#active.clear();
    this.#statuses = [];
  }

  #register(binding: GlobalShortcutBinding): GlobalShortcutRegistration {
    const previous = this.#active.get(binding.command_id);
    if (previous?.chord === binding.chord) {
      return this.#registered(binding, previous.chord);
    }
    if (!this.#accessibility.allowed) {
      return this.#failed(binding, "failed_permission", previous, this.#accessibility.settingsURL);
    }

    let accelerator: string;
    try {
      accelerator = chordToAccelerator(binding.chord);
    } catch (error) {
      if (!(error instanceof UnconvertibleShortcutError)) throw error;
      return this.#failed(binding, "unsupported", previous);
    }

    if (previous) this.#globalShortcut.unregister(previous.accelerator);
    if (this.#globalShortcut.register(accelerator, () => this.#onInvoke(binding.command_id))) {
      this.#active.set(binding.command_id, { chord: binding.chord, accelerator });
      return this.#registered(binding, binding.chord);
    }

    const restored = this.#restore(binding.command_id, previous);
    return this.#failed(binding, "failed_in_use", restored ? previous : undefined);
  }

  #restore(commandID: string, previous: ActiveBinding | undefined): boolean {
    if (!previous) {
      this.#active.delete(commandID);
      return false;
    }
    const restored = this.#globalShortcut.register(previous.accelerator, () =>
      this.#onInvoke(commandID)
    );
    if (restored) {
      this.#active.set(commandID, previous);
      return true;
    }
    this.#active.delete(commandID);
    return false;
  }

  #registered(binding: GlobalShortcutBinding, activeChord: string): GlobalShortcutRegistration {
    return {
      command_id: binding.command_id,
      intended_chord: binding.chord,
      active_chord: activeChord,
      status: "registered",
    };
  }

  #failed(
    binding: GlobalShortcutBinding,
    status: "failed_in_use" | "failed_permission" | "unsupported",
    previous: ActiveBinding | undefined,
    settingsURL?: string
  ): GlobalShortcutRegistration {
    return {
      command_id: binding.command_id,
      intended_chord: binding.chord,
      status,
      ...(previous ? { active_chord: previous.chord } : {}),
      ...(status === "failed_in_use"
        ? { reason: "unavailable — in use by another application" }
        : {}),
      ...(status === "failed_permission"
        ? { reason: "Accessibility permission is required." }
        : {}),
      ...(status === "unsupported" ? { reason: "The shortcut is not supported by Electron." } : {}),
      ...(settingsURL ? { settings_url: settingsURL } : {}),
    };
  }
}
