import { createAtom } from "@xstate/store";

import type {
  UnsavedCloseQuestion,
  UnsavedWindowEdits,
  UnsavedWindowEditsPort,
} from "@/hooks/use-window-navigation-guard";

import type { WindowCloseGuard } from "./window-close-targets";

/**
 * Operator close admission for windows whose editors hold unsaved edits (#1653). Editors hold
 * their windows while dirty; a close covering held windows asks once for all of them. Keep
 * editing cancels the whole close; Discard lets it go on, and the edits are dropped only once the
 * close went through (`settled`), so a later guard or a refused command leaves them intact.
 * View-only closes never reach a guard: the editors keep their drafts when they unmount.
 */
export class UnsavedWindowClose implements UnsavedWindowEditsPort {
  readonly question = createAtom<UnsavedCloseQuestion | null>(null);
  private readonly held = new Map<string, Set<UnsavedWindowEdits>>();
  private asking: ReadonlySet<string> = new Set();
  private confirmed: readonly UnsavedWindowEdits[] = [];

  hold(windowId: string, edits: UnsavedWindowEdits): () => void {
    const set = this.held.get(windowId) ?? new Set<UnsavedWindowEdits>();
    set.add(edits);
    this.held.set(windowId, set);
    return () => {
      set.delete(edits);
      if (set.size === 0 && this.held.get(windowId) === set) this.held.delete(windowId);
      // Edits leaving a window the question covers change what it asks about: start over.
      if (this.asking.has(windowId)) this.cancel();
    };
  }

  /** Answers a pending question with Keep editing, as when its shell binding goes. */
  cancel(): void {
    this.question.get()?.answer(false);
  }

  readonly guard: WindowCloseGuard = async (targets, isCurrent) => {
    const windowIds = targets.flatMap(window => (this.held.has(window.id) ? [window.id] : []));
    const presenter = windowIds.length === 0 ? undefined : this.held.get(windowIds[0]!);
    const first = presenter?.values().next().value;
    if (first === undefined) return true;
    const discard = await new Promise<boolean>(resolve => {
      this.asking = new Set(windowIds);
      this.question.set({
        presenter: first,
        windowCount: windowIds.length,
        answer: value => {
          this.asking = new Set();
          this.question.set(null);
          resolve(value);
        },
      });
    });
    if (!discard || !isCurrent()) return false;
    this.confirmed = windowIds.flatMap(id => Array.from(this.held.get(id) ?? []));
    return true;
  };

  /** Drops the edits a Discard answered for, once their windows closed. */
  readonly settled = (closed: boolean): void => {
    const confirmed = this.confirmed;
    this.confirmed = [];
    if (closed) for (const edits of confirmed) edits.discard();
  };
}

/** Admits a close only when every guard does, asking them in order and stopping at a refusal. */
export function composeCloseGuards(...guards: readonly WindowCloseGuard[]): WindowCloseGuard {
  return async (targets, isCurrent) => {
    for (const guard of guards) {
      if (!(await guard(targets, isCurrent)) || !isCurrent()) return false;
    }
    return true;
  };
}
