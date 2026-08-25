/**
 * Terminal instances live here, not in React state.
 *
 * A terminal's scrollback is the only copy of what a viewer already saw, so it
 * must outlive the component that shows it: switching tabs unmounts a pane, and
 * an instance owned by React state would take the buffer with it. Entries are
 * therefore released (unmounted) and destroyed (disposed) through separate
 * calls — only the owner of the terminal's lifetime decides the second one.
 *
 * The emulator is opened into a registry-owned host element. Re-attaching means
 * moving that node into the new container; the node is simply detached while no
 * pane shows it, which is why there is no hidden parking container anywhere in
 * the document.
 *
 * Emulator subscriptions are owned by the instance and fan out to whichever
 * views are mounted right now. Binding them to one mount would leave a
 * reattached buffer talking to the component that unmounted.
 */

import type { Terminal } from "@xterm/xterm";

import type { TerminalFitAddon } from "./terminal-engine";
import type { TerminalRendererBinding, TerminalRendererKind } from "./terminal-renderer";

/** What one mounted view wants to hear from the emulator it is showing. */
export interface TerminalInstanceListener {
  onData(data: string): void;
  onSelectionChange(selection: string): void;
  onRendererChange(renderer: TerminalRendererKind): void;
}

export interface TerminalInstance {
  readonly key: string;
  readonly terminal: Terminal;
  readonly fit: TerminalFitAddon;
  readonly host: HTMLElement;
  renderer: TerminalRendererBinding;
  /** How many mounted views currently show this instance. */
  mountCount: number;
  /** The views listening right now. Mount-scoped, never instance-scoped. */
  readonly listeners: Set<TerminalInstanceListener>;
  /** Torn down with the instance: emulator subscriptions, theme observer. */
  readonly teardown: Array<() => void>;
}

const instances = new Map<string, TerminalInstance>();

export function getTerminalInstance(key: string): TerminalInstance | undefined {
  return instances.get(key);
}

export function registerTerminalInstance(instance: TerminalInstance): TerminalInstance {
  instances.set(instance.key, instance);
  return instance;
}

/**
 * Subscribes one mounted view and returns its own unsubscribe.
 *
 * Every mount adds and removes only its own listener, so a StrictMode double
 * mount, a tab switch, and two panes on one terminal all behave the same.
 */
export function addTerminalInstanceListener(
  instance: TerminalInstance,
  listener: TerminalInstanceListener
): () => void {
  instance.listeners.add(listener);
  return () => {
    instance.listeners.delete(listener);
  };
}

/** Fans an emulator event out to every view showing this instance. */
export function notifyTerminalInstance(
  instance: TerminalInstance,
  notify: (listener: TerminalInstanceListener) => void
): void {
  for (const listener of Array.from(instance.listeners)) {
    notify(listener);
  }
}

/**
 * Marks the instance as shown by one more view and moves its host into
 * `container`. Appending a node that already has a parent moves it, so a
 * StrictMode double-mount is a no-op rather than a duplicated terminal.
 */
export function attachTerminalInstance(instance: TerminalInstance, container: HTMLElement): void {
  instance.mountCount += 1;
  if (instance.host.parentElement !== container) {
    container.appendChild(instance.host);
  }
}

/**
 * Marks the instance as shown by one fewer view and detaches its host when the
 * last one goes. The buffer, the renderer and the emulator all survive.
 */
export function detachTerminalInstance(instance: TerminalInstance): void {
  instance.mountCount = Math.max(0, instance.mountCount - 1);
  if (instance.mountCount === 0) {
    instance.host.remove();
  }
}

/** Disposes one instance for good. Called when its terminal itself is gone. */
export function destroyTerminalInstance(key: string): void {
  const instance = instances.get(key);
  if (!instance) return;
  instances.delete(key);
  instance.listeners.clear();
  for (const teardown of instance.teardown) {
    teardown();
  }
  instance.renderer.dispose();
  instance.terminal.dispose();
  instance.host.remove();
}

/**
 * Disposes every instance whose key the predicate selects.
 *
 * Profile switching uses this: the previous profile's terminals are not merely
 * hidden from the client, their buffers leave the process.
 */
export function destroyTerminalInstances(predicate: (key: string) => boolean): void {
  for (const key of Array.from(instances.keys())) {
    if (predicate(key)) destroyTerminalInstance(key);
  }
}

/** The renderer currently painting this instance. */
export function terminalRendererKind(key: string): TerminalRendererKind | undefined {
  return instances.get(key)?.renderer.kind;
}
